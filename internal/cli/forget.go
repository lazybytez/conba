package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/forget"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/lazybytez/conba/internal/runtime/docker"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

// errEmptyGlobalRetention is returned when surgical forget mode is invoked
// without any retention policy configured. Surgical mode bypasses
// per-target labels, so a missing global policy leaves no policy to apply.
var errEmptyGlobalRetention = errors.New(
	"surgical forget requires a retention: block in config; none found",
)

// forgetFlags holds the CLI-provided flags for the forget command.
type forgetFlags struct {
	dryRun    bool
	noPrune   bool
	allHosts  bool
	container string
	volume    string
	tags      []string
}

// surgical reports whether the user supplied any of the tag-based filters
// that switch forget into surgical (single-snapshot-set) mode.
func (f forgetFlags) surgical() bool {
	return f.container != "" || f.volume != "" || len(f.tags) > 0
}

// forgetRequest bundles the dependencies and inputs shared by the
// surgical and discovery-driven forget paths.
type forgetRequest struct {
	cmd      *cobra.Command
	cfg      *config.Config
	logger   *zap.Logger
	client   *restic.Client
	hostname string
	flags    forgetFlags
}

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

	flags := readForgetFlags(cmd.Flags())

	client, err := restic.New(cfg.Restic, logger)
	if err != nil {
		return fmt.Errorf("create restic client: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}

	req := forgetRequest{
		cmd:      cmd,
		cfg:      cfg,
		logger:   logger,
		client:   client,
		hostname: hostname,
		flags:    flags,
	}

	if flags.surgical() {
		return runForgetSurgical(req)
	}

	return runForgetDiscovery(req)
}

// readForgetFlags reads the user-provided forget flags into a struct.
func readForgetFlags(flags *pflag.FlagSet) forgetFlags {
	return forgetFlags{
		dryRun:    flagBool(flags, "dry-run"),
		noPrune:   flagBool(flags, "no-prune"),
		allHosts:  flagBool(flags, "all-hosts"),
		container: flagString(flags, "container"),
		volume:    flagString(flags, "volume"),
		tags:      flagStringArray(flags, "tag"),
	}
}

func runForgetSurgical(req forgetRequest) error {
	if req.cfg.Retention.IsEmpty() {
		return errEmptyGlobalRetention
	}

	tags := buildSurgicalTags(req.flags, req.hostname)
	policy := forget.ToResticPolicy(req.cfg.Retention)
	opts := restic.ForgetOptions{Prune: !req.flags.noPrune, DryRun: req.flags.dryRun}

	err := req.client.Forget(req.cmd.Context(), tags, policy, opts)
	if err != nil {
		return fmt.Errorf("surgical forget: %w", err)
	}

	message := "Surgical forget complete."
	if req.flags.dryRun {
		message = "Surgical forget complete (dry-run)."
	}

	_, writeErr := fmt.Fprintln(req.cmd.OutOrStdout(), message)
	if writeErr != nil {
		return fmt.Errorf("writing output: %w", writeErr)
	}

	return nil
}

func buildSurgicalTags(flags forgetFlags, hostname string) []string {
	var tags []string

	if flags.container != "" {
		tags = append(tags, backup.ContainerTagPrefix+flags.container)
	}

	if flags.volume != "" {
		tags = append(tags, backup.VolumeTagPrefix+flags.volume)
	}

	tags = append(tags, flags.tags...)

	if !flags.allHosts {
		tags = append(tags, backup.HostTagPrefix+hostname)
	}

	return tags
}

func runForgetDiscovery(req forgetRequest) error {
	ctx := req.cmd.Context()

	runtime, cleanup, err := connectDockerForForget(ctx, req.cfg, req.logger)
	if err != nil {
		return err
	}

	defer cleanup()

	targets, err := discovery.Discover(ctx, runtime)
	if err != nil {
		return fmt.Errorf("discover volumes: %w", err)
	}

	result := filter.Apply(targets, req.cfg.Discovery)

	if len(result.Included) == 0 {
		_, writeErr := fmt.Fprintln(req.cmd.OutOrStdout(), "No volumes to forget.")
		if writeErr != nil {
			return fmt.Errorf("writing output: %w", writeErr)
		}

		return nil
	}

	opts := forget.Options{
		Hostname: req.hostname,
		AllHosts: req.flags.allHosts,
		DryRun:   req.flags.dryRun,
		Prune:    !req.flags.noPrune,
	}

	err = forget.Run(
		ctx,
		result.Included,
		req.client.Forget,
		req.cfg.Retention,
		opts,
		req.cmd.OutOrStdout(),
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
