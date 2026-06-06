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

	targets, runtimeClient, cleanup, err := discoverFiltered(ctx, cfg, logger)
	if err != nil {
		return err
	}

	// Keep the docker client open for the full backup: backup.Run uses it as
	// the Execer to run pre-backup commands via the Docker SDK.
	defer cleanup()

	if len(targets) == 0 {
		_, writeErr := fmt.Fprintln(cmd.OutOrStdout(), "No volumes to back up.")
		if writeErr != nil {
			return fmt.Errorf("writing output: %w", writeErr)
		}

		return nil
	}

	if dryRun {
		return printDryRun(cmd.OutOrStdout(), targets, cfg.PreBackupCommands.Enabled)
	}

	if cfg.PreBackupCommands.Enabled {
		err = printPreBackupSummary(cmd.OutOrStdout(), targets)
		if err != nil {
			return err
		}
	}

	return executeBackup(cmd, cfg, logger, runtimeClient, targets)
}

func executeBackup(
	cmd *cobra.Command,
	cfg *config.Config,
	logger *zap.Logger,
	runtimeClient *docker.Client,
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

	opts := buildBackupOptions(client, runtimeClient, cfg, hostname)

	err = backup.Run(cmd.Context(), targets, opts, cmd.OutOrStdout())
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
