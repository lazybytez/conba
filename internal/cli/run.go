package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/forget"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// NewRunCommand creates the run subcommand that executes the standard
// backup cycle in sequence: init, backup, forget. It is intended for
// CI/CD environments where a single command is preferable to chaining
// the three phases.
func NewRunCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the standard cycle: init, backup, forget",
		RunE:  runRun,
	}

	cmd.Flags().Bool("dry-run", false,
		"show what would happen without making changes")
	cmd.Flags().Bool("all-hosts", false,
		"forget across all hostnames in the repo (default: current host only)")
	cmd.Flags().Bool("no-forget", false,
		"skip the forget phase (run init and backup only)")

	return cmd
}

func runRun(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := config.FromContext(ctx)
	logger := logging.FromContext(ctx)

	if cfg == nil {
		return errMissingConfig
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	allHosts, _ := cmd.Flags().GetBool("all-hosts")
	noForget, _ := cmd.Flags().GetBool("no-forget")

	client, err := restic.New(cfg.Restic, logger)
	if err != nil {
		return fmt.Errorf("create restic client: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}

	out := cmd.OutOrStdout()

	err = runInitPhase(ctx, out, client)
	if err != nil {
		return err
	}

	targets, cleanup, err := runBackupPhase(ctx, out, cfg, logger, client, hostname, dryRun)
	if err != nil {
		return err
	}

	defer cleanup()

	if noForget {
		return nil
	}

	return runForgetPhase(ctx, out, cfg, client, hostname, targets, allHosts, dryRun)
}

func runInitPhase(ctx context.Context, out io.Writer, client *restic.Client) error {
	err := writePhaseHeader(out, "init")
	if err != nil {
		return err
	}

	err = client.Init(ctx)
	if err != nil {
		return fmt.Errorf("run init: %w", err)
	}

	_, err = fmt.Fprintln(out, "Repository initialized.")
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

// runBackupPhase prints the backup header, opens the docker connection,
// discovers and filters targets, and runs the backup. The returned
// cleanup func closes the docker client and is always non-nil so the
// caller can defer it unconditionally.
func runBackupPhase(
	ctx context.Context,
	out io.Writer,
	cfg *config.Config,
	logger *zap.Logger,
	client *restic.Client,
	hostname string,
	dryRun bool,
) ([]discovery.Target, func(), error) {
	err := writePhaseHeader(out, "backup")
	if err != nil {
		return nil, func() {}, err
	}

	runtime, cleanup, err := connectDockerForForget(ctx, cfg, logger)
	if err != nil {
		return nil, func() {}, err
	}

	targets, err := discovery.Discover(ctx, runtime)
	if err != nil {
		cleanup()

		return nil, func() {}, fmt.Errorf("discover volumes: %w", err)
	}

	result := filter.Apply(targets, cfg.Discovery)

	if len(result.Included) == 0 {
		_, writeErr := fmt.Fprintln(out, "No volumes to back up.")
		if writeErr != nil {
			cleanup()

			return nil, func() {}, fmt.Errorf("writing output: %w", writeErr)
		}

		return nil, cleanup, nil
	}

	if dryRun {
		err = printDryRun(out, result.Included)
		if err != nil {
			cleanup()

			return nil, func() {}, err
		}

		return result.Included, cleanup, nil
	}

	err = backup.Run(ctx, result.Included, client.Backup, hostname, out)
	if err != nil {
		cleanup()

		return nil, func() {}, fmt.Errorf("run backup: %w", err)
	}

	return result.Included, cleanup, nil
}

func runForgetPhase(
	ctx context.Context,
	out io.Writer,
	cfg *config.Config,
	client *restic.Client,
	hostname string,
	targets []discovery.Target,
	allHosts bool,
	dryRun bool,
) error {
	err := writePhaseHeader(out, "forget")
	if err != nil {
		return err
	}

	if len(targets) == 0 {
		_, writeErr := fmt.Fprintln(out, "No volumes to forget.")
		if writeErr != nil {
			return fmt.Errorf("writing output: %w", writeErr)
		}

		return nil
	}

	opts := forget.Options{
		Hostname: hostname,
		AllHosts: allHosts,
		DryRun:   dryRun,
		Prune:    true,
	}

	err = forget.Run(ctx, targets, client.Forget, cfg.Retention, opts, out)
	if err != nil {
		return fmt.Errorf("run forget: %w", err)
	}

	return nil
}

func writePhaseHeader(out io.Writer, phase string) error {
	_, err := fmt.Fprintf(out, "==> %s\n", phase)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}
