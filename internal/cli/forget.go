package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/forget"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/lazybytez/conba/internal/runtime/docker"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// errEmptyGlobalRetention is returned when surgical forget mode is invoked
// without any retention policy configured. Surgical mode bypasses
// per-target labels, so a missing global policy leaves no policy to apply.
var errEmptyGlobalRetention = errors.New(
	"surgical forget requires a retention: block in config; none found",
)

// NewForgetCommand creates the forget subcommand that applies retention
// policies to existing snapshots and (optionally) prunes the repository.
func NewForgetCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forget",
		Short: "Apply snapshot retention policies and (optionally) prune",
		RunE:  runForget,
	}

	cmd.Flags().Bool("dry-run", false,
		"show what would be forgotten without applying changes")
	cmd.Flags().Bool("no-prune", false,
		"skip the prune step (forget references only, do not reclaim disk)")
	cmd.Flags().Bool("all-hosts", false,
		"operate across all hostnames in the repo (default: current host only)")
	cmd.Flags().String("container", "",
		"surgical: restrict to snapshots tagged container=<name>")
	cmd.Flags().String("volume", "",
		"surgical: restrict to snapshots tagged volume=<name>")
	cmd.Flags().StringArray("tag", nil,
		"surgical: restrict to snapshots tagged <key>=<value> (repeatable)")

	return cmd
}

func runForget(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := config.FromContext(ctx)
	logger := logging.FromContext(ctx)

	if cfg == nil {
		return errMissingConfig
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	noPrune, _ := cmd.Flags().GetBool("no-prune")
	allHosts, _ := cmd.Flags().GetBool("all-hosts")
	containerFlag, _ := cmd.Flags().GetString("container")
	volumeFlag, _ := cmd.Flags().GetString("volume")
	tagFlags, _ := cmd.Flags().GetStringArray("tag")

	surgical := containerFlag != "" || volumeFlag != "" || len(tagFlags) > 0

	client, err := restic.New(cfg.Restic, logger)
	if err != nil {
		return fmt.Errorf("create restic client: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}

	if surgical {
		return runForgetSurgical(
			cmd, cfg, client, hostname,
			containerFlag, volumeFlag, tagFlags,
			allHosts, noPrune, dryRun,
		)
	}

	return runForgetDiscovery(
		cmd, cfg, logger, client, hostname, allHosts, noPrune, dryRun,
	)
}

func runForgetSurgical(
	cmd *cobra.Command,
	cfg *config.Config,
	client *restic.Client,
	hostname string,
	containerFlag string,
	volumeFlag string,
	tagFlags []string,
	allHosts bool,
	noPrune bool,
	dryRun bool,
) error {
	if cfg.Retention.KeepDaily+cfg.Retention.KeepWeekly+
		cfg.Retention.KeepMonthly+cfg.Retention.KeepYearly == 0 {
		return errEmptyGlobalRetention
	}

	tags := buildSurgicalTags(containerFlag, volumeFlag, tagFlags, hostname, allHosts)

	policy := restic.ForgetPolicy{
		KeepDaily:   cfg.Retention.KeepDaily,
		KeepWeekly:  cfg.Retention.KeepWeekly,
		KeepMonthly: cfg.Retention.KeepMonthly,
		KeepYearly:  cfg.Retention.KeepYearly,
	}

	opts := restic.ForgetOptions{Prune: !noPrune, DryRun: dryRun}

	err := client.Forget(cmd.Context(), tags, policy, opts)
	if err != nil {
		return fmt.Errorf("surgical forget: %w", err)
	}

	out := cmd.OutOrStdout()
	if dryRun {
		_, writeErr := fmt.Fprintln(out,
			"Forget complete (dry-run): 1 would succeed, 0 skipped, 0 failed.")
		if writeErr != nil {
			return fmt.Errorf("writing output: %w", writeErr)
		}

		return nil
	}

	_, writeErr := fmt.Fprintln(out,
		"Forget complete: 1 succeeded, 0 skipped, 0 failed.")
	if writeErr != nil {
		return fmt.Errorf("writing output: %w", writeErr)
	}

	return nil
}

func buildSurgicalTags(
	containerFlag string,
	volumeFlag string,
	tagFlags []string,
	hostname string,
	allHosts bool,
) []string {
	var tags []string

	if containerFlag != "" {
		tags = append(tags, "container="+containerFlag)
	}

	if volumeFlag != "" {
		tags = append(tags, "volume="+volumeFlag)
	}

	tags = append(tags, tagFlags...)

	if !allHosts {
		tags = append(tags, "hostname="+hostname)
	}

	return tags
}

func runForgetDiscovery(
	cmd *cobra.Command,
	cfg *config.Config,
	logger *zap.Logger,
	client *restic.Client,
	hostname string,
	allHosts bool,
	noPrune bool,
	dryRun bool,
) error {
	ctx := cmd.Context()

	runtime, cleanup, err := connectDockerForForget(ctx, cfg, logger)
	if err != nil {
		return err
	}

	defer cleanup()

	targets, err := discovery.Discover(ctx, runtime)
	if err != nil {
		return fmt.Errorf("discover volumes: %w", err)
	}

	result := filter.Apply(targets, cfg.Discovery)

	if len(result.Included) == 0 {
		_, writeErr := fmt.Fprintln(cmd.OutOrStdout(), "No volumes to forget.")
		if writeErr != nil {
			return fmt.Errorf("writing output: %w", writeErr)
		}

		return nil
	}

	opts := forget.Options{
		Hostname: hostname,
		AllHosts: allHosts,
		DryRun:   dryRun,
		Prune:    !noPrune,
	}

	err = forget.Run(
		ctx,
		result.Included,
		client.Forget,
		cfg.Retention,
		opts,
		cmd.OutOrStdout(),
	)
	if err != nil {
		return fmt.Errorf("run forget: %w", err)
	}

	return nil
}

func connectDockerForForget(
	ctx context.Context,
	cfg *config.Config,
	logger *zap.Logger,
) (*docker.Client, func(), error) {
	logger.Debug("connecting to docker",
		zap.String("host", cfg.Runtime.Docker.Host))

	runtime, err := docker.New(ctx, cfg.Runtime.Docker.Host)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to docker: %w", err)
	}

	cleanup := func() {
		closeErr := runtime.Close()
		if closeErr != nil {
			logger.Warn("failed to close docker client",
				zap.Error(closeErr))
		}
	}

	return runtime, cleanup, nil
}
