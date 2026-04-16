package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/lazybytez/conba/internal/runtime/docker"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// NewBackupCommand creates the backup subcommand that backs up
// container volumes via restic.
func NewBackupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Back up container volumes",
		RunE:  runBackup,
	}

	cmd.Flags().Bool("dry-run", false, "show what would be backed up without running")

	return cmd
}

func runBackup(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := config.FromContext(ctx)
	logger := logging.FromContext(ctx)

	if cfg == nil {
		return errMissingConfig
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")

	logger.Debug("connecting to docker",
		zap.String("host", cfg.Runtime.Docker.Host))

	runtime, err := docker.New(ctx, cfg.Runtime.Docker.Host)
	if err != nil {
		return fmt.Errorf("connect to docker: %w", err)
	}

	defer func() {
		closeErr := runtime.Close()
		if closeErr != nil {
			logger.Warn("failed to close docker client",
				zap.Error(closeErr))
		}
	}()

	targets, err := discovery.Discover(ctx, runtime)
	if err != nil {
		return fmt.Errorf("discover volumes: %w", err)
	}

	result := filter.Apply(targets, cfg.Discovery)

	if len(result.Included) == 0 {
		_, writeErr := fmt.Fprintln(cmd.OutOrStdout(), "No volumes to back up.")
		if writeErr != nil {
			return fmt.Errorf("writing output: %w", writeErr)
		}

		return nil
	}

	if dryRun {
		return printDryRun(cmd.OutOrStdout(), result.Included)
	}

	return executeBackup(cmd, cfg, logger, result.Included)
}

func executeBackup(
	cmd *cobra.Command,
	cfg *config.Config,
	logger *zap.Logger,
	targets []discovery.Target,
) error {
	client, err := restic.New(cfg.Restic, logger)
	if err != nil {
		return fmt.Errorf("create restic client: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}

	err = backup.Run(cmd.Context(), targets, client.Backup, hostname, cmd.OutOrStdout())
	if err != nil {
		return fmt.Errorf("run backup: %w", err)
	}

	return nil
}

func printDryRun(out io.Writer, targets []discovery.Target) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}

	for _, target := range targets {
		tags := backup.BuildTags(target, hostname)

		_, err = fmt.Fprintf(out, "%s (%s)\n",
			target.Container.Name,
			shortID(target.Container.ID))
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}

		_, err = fmt.Fprintf(out, "  %s  %s \u2192 %s\n",
			target.Mount.Type,
			target.Mount.Name,
			target.Mount.Source)
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}

		_, err = fmt.Fprintf(out, "  tags: %s\n",
			formatTags(tags))
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}

		_, err = fmt.Fprintln(out)
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	}

	_, err = fmt.Fprintf(out, "%d volume(s) would be backed up.\n", len(targets))
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

func formatTags(tags []string) string {
	return strings.Join(tags, ", ")
}
