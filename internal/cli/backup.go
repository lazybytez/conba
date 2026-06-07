package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/report"
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

	dryRun := flagBool(cmd.Flags(), "dry-run")
	reporter := report.FromContext(ctx)

	targets, runtimeClient, cleanup, err := discoverFiltered(ctx, cfg, logger)
	if err != nil {
		return err
	}

	// Keep the docker client open for the full backup: backup.Run uses it as
	// the Execer to run pre-backup commands via the Docker SDK.
	defer cleanup()

	if len(targets) == 0 {
		reporter.Emit(report.Event{
			Level:   report.LevelInfo,
			Name:    "backup.summary",
			Message: "No volumes to back up.",
			Fields: []report.Field{
				report.F("succeeded", 0),
				report.F("skipped", 0),
				report.F("failed", 0),
			},
		})

		return nil
	}

	if dryRun {
		return runBackupDryRun(reporter, targets, cfg.PreBackupCommands.Enabled)
	}

	if cfg.PreBackupCommands.Enabled {
		err = reportPreBackupSummary(reporter, targets)
		if err != nil {
			return err
		}
	}

	return executeBackup(cmd, cfg, logger, runtimeClient, targets, reporter)
}

// runBackupDryRun renders the dry-run plan as structured events (json) or the
// human plan (text).
func runBackupDryRun(
	reporter report.Reporter,
	targets []discovery.Target,
	preBackupEnabled bool,
) error {
	if reporter.Mode() == report.ModeJSON {
		emitDryRun(reporter, targets, preBackupEnabled)

		return nil
	}

	return printDryRun(reporter.Out(), targets, preBackupEnabled)
}

// reportPreBackupSummary emits the pre-backup banner as structured events
// (json) or human lines (text).
func reportPreBackupSummary(reporter report.Reporter, targets []discovery.Target) error {
	if reporter.Mode() != report.ModeJSON {
		return printPreBackupSummary(reporter.Out(), targets)
	}

	seen := make(map[string]struct{})

	for _, target := range targets {
		name := target.Container.Name
		if _, ok := seen[name]; ok {
			continue
		}

		spec, hasSpec, err := filter.PreBackup(target)
		if err != nil || !hasSpec {
			continue
		}

		seen[name] = struct{}{}

		filename := spec.Filename
		if filename == "" {
			filename = name
		}

		reporter.Emit(report.Event{
			Level: report.LevelInfo,
			Name:  "pre_backup.plan",
			Fields: []report.Field{
				report.F("container", name),
				report.F("mode", string(spec.Mode)),
				report.F("filename", filename),
			},
		})
	}

	return nil
}

func executeBackup(
	cmd *cobra.Command,
	cfg *config.Config,
	logger *zap.Logger,
	runtimeClient *docker.Client,
	targets []discovery.Target,
	reporter report.Reporter,
) error {
	client, err := restic.New(cfg.Restic, logger)
	if err != nil {
		return fmt.Errorf("create restic client: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}

	opts := buildBackupOptions(client, runtimeClient, cfg, hostname)

	err = backup.Run(cmd.Context(), targets, opts, reporter)
	if err != nil {
		return fmt.Errorf("run backup: %w", err)
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
// "default to labeled container name" rule for filename.
func writePreBackupLine(out io.Writer, container string, spec filter.Spec) error {
	filename := spec.Filename
	if filename == "" {
		filename = container
	}

	_, err := fmt.Fprintf(
		out,
		"pre-backup: %s mode=%s filename=%s\n",
		container,
		spec.Mode,
		filename,
	)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}
