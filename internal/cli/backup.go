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
		return runDryRun(cmd.OutOrStdout(), result.Included, cfg.PreBackupCommands.Enabled)
	}

	if cfg.PreBackupCommands.Enabled {
		err = printPreBackupSummary(cmd.OutOrStdout(), result.Included)
		if err != nil {
			return err
		}
	}

	return executeBackup(cmd, cfg, logger, result.Included)
}

// runDryRun routes dry-run output through the pre-backup-aware renderer
// when the feature flag is enabled, otherwise emits the legacy listing.
func runDryRun(out io.Writer, targets []discovery.Target, preBackupEnabled bool) error {
	if preBackupEnabled {
		return printDryRunWithPreBackup(out, targets)
	}

	return printDryRun(out, targets)
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

	err = backup.Run(
		cmd.Context(),
		targets,
		client.Backup,
		client.BackupFromCommand,
		cfg.PreBackupCommands.Enabled,
		hostname,
		cmd.OutOrStdout(),
	)
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

// printDryRunWithPreBackup is the dry-run renderer used when the
// pre_backup_commands feature flag is enabled. It emits one
// "would run: <cmd> in <execContainer>" line per labeled container, and
// either replaces the volume listing with a "would skip" line (replace
// mode) or keeps the legacy listing alongside the run line (alongside
// mode). Unlabeled containers and containers with invalid labels render
// exactly as the legacy printDryRun does.
func printDryRunWithPreBackup(out io.Writer, targets []discovery.Target) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}

	var volumeCount int

	for _, group := range groupByContainer(targets) {
		count, groupErr := writeDryRunGroup(out, group, hostname)
		if groupErr != nil {
			return groupErr
		}

		volumeCount += count
	}

	_, err = fmt.Fprintf(out, "%d volume(s) would be backed up.\n", volumeCount)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

// writeDryRunGroup renders one container group's dry-run output, returning
// the number of volume targets that would be backed up (i.e. excluding
// replaced mounts). Containers without pre-backup labels render via the
// legacy listing; labeled containers emit a "would run:" line followed by
// per-mount skip/listing depending on mode.
func writeDryRunGroup(
	out io.Writer,
	group []discovery.Target,
	hostname string,
) (int, error) {
	first := group[0]

	spec, hasSpec, err := filter.PreBackup(first)
	if err != nil || !hasSpec {
		return writeDryRunLegacyGroup(out, group, hostname)
	}

	execContainer := spec.Container
	if execContainer == "" {
		execContainer = first.Container.Name
	}

	_, runErr := fmt.Fprintf(
		out,
		"would run: %s in %s\n",
		spec.Command,
		execContainer,
	)
	if runErr != nil {
		return 0, fmt.Errorf("writing output: %w", runErr)
	}

	if spec.Mode == filter.ModeReplace {
		writeErr := writeDryRunReplaceSkips(out, group)
		if writeErr != nil {
			return 0, writeErr
		}

		return 0, nil
	}

	// Alongside mode keeps the legacy volume listing for each mount.
	count, listErr := writeDryRunLegacyGroup(out, group, hostname)
	if listErr != nil {
		return 0, listErr
	}

	return count, nil
}

// writeDryRunReplaceSkips emits one "would skip" line per mount in a
// replace-mode group and a trailing blank line for spacing.
func writeDryRunReplaceSkips(out io.Writer, group []discovery.Target) error {
	for _, target := range group {
		_, err := fmt.Fprintf(
			out,
			"would skip: %s/%s — replaced by pre-backup stream\n",
			target.Container.Name,
			target.Mount.Name,
		)
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	}

	_, err := fmt.Fprintln(out)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

// writeDryRunLegacyGroup renders the legacy per-target dry-run listing for
// one container group, returning the number of mounts written.
func writeDryRunLegacyGroup(
	out io.Writer,
	group []discovery.Target,
	hostname string,
) (int, error) {
	for _, target := range group {
		err := writeDryRunTarget(out, target, hostname)
		if err != nil {
			return 0, err
		}
	}

	return len(group), nil
}

// writeDryRunTarget renders one target's legacy dry-run block: header,
// mount line, tag list, and trailing blank line.
func writeDryRunTarget(out io.Writer, target discovery.Target, hostname string) error {
	tags := backup.BuildTags(target, hostname)

	_, err := fmt.Fprintf(out, "%s (%s)\n",
		target.Container.Name,
		shortID(target.Container.ID))
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	_, err = fmt.Fprintf(out, "  %s  %s → %s\n",
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

	return nil
}

// printPreBackupSummary emits one summary line per unique container that
// carries valid pre-backup labels. Targets whose labels fail to parse are
// silently skipped here; the downstream backup.Run reports the failure.
func printPreBackupSummary(out io.Writer, targets []discovery.Target) error {
	seen := make(map[string]struct{})

	for _, target := range targets {
		name := target.Container.Name
		if _, ok := seen[name]; ok {
			continue
		}

		spec, hasSpec, err := filter.PreBackup(target)
		// Invalid-mode targets surface their failure in runGroup's output
		// during the actual backup; suppressing the summary line here keeps
		// the pre-run banner clean of redundant errors.
		if err != nil || !hasSpec {
			continue
		}

		seen[name] = struct{}{}

		writeErr := writePreBackupLine(out, name, spec)
		if writeErr != nil {
			return writeErr
		}
	}

	return nil
}

// writePreBackupLine writes a single pre-backup summary line, applying the
// "default to labeled container name" rules for exec and filename.
func writePreBackupLine(out io.Writer, container string, spec filter.Spec) error {
	execContainer := spec.Container
	if execContainer == "" {
		execContainer = container
	}

	filename := spec.Filename
	if filename == "" {
		filename = container
	}

	_, err := fmt.Fprintf(
		out,
		"pre-backup: %s mode=%s exec=%s filename=%s\n",
		container,
		spec.Mode,
		execContainer,
		filename,
	)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}
