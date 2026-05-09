package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/forget"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// runFlags holds the CLI-provided flags for the run command.
type runFlags struct {
	dryRun   bool
	allHosts bool
	noForget bool
}

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

	flags := readRunFlags(cmd.Flags())

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

	targets, cleanup, err := discoverFiltered(ctx, cfg, logger)
	if err != nil {
		return err
	}

	defer cleanup()

	if len(targets) == 0 {
		_, writeErr := fmt.Fprintln(out,
			"No volumes discovered; nothing to back up or forget.")
		if writeErr != nil {
			return fmt.Errorf("writing output: %w", writeErr)
		}

		return nil
	}

	err = runBackupPhase(ctx, out, client, hostname, targets, flags.dryRun)
	if err != nil {
		return err
	}

	if flags.noForget {
		return nil
	}

	return runForgetPhase(ctx, out, cfg, client, hostname, targets, flags)
}

func readRunFlags(flags *pflag.FlagSet) runFlags {
	return runFlags{
		dryRun:   flagBool(flags, "dry-run"),
		allHosts: flagBool(flags, "all-hosts"),
		noForget: flagBool(flags, "no-forget"),
	}
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

func runBackupPhase(
	ctx context.Context,
	out io.Writer,
	client *restic.Client,
	hostname string,
	targets []discovery.Target,
	dryRun bool,
) error {
	err := writePhaseHeader(out, "backup")
	if err != nil {
		return err
	}

	if dryRun {
		return printDryRun(out, targets)
	}

	err = backup.Run(ctx, targets, client.Backup, hostname, out)
	if err != nil {
		return fmt.Errorf("run backup: %w", err)
	}

	return nil
}

func runForgetPhase(
	ctx context.Context,
	out io.Writer,
	cfg *config.Config,
	client *restic.Client,
	hostname string,
	targets []discovery.Target,
	flags runFlags,
) error {
	err := writePhaseHeader(out, "forget")
	if err != nil {
		return err
	}

	opts := forget.Options{
		Hostname: hostname,
		AllHosts: flags.allHosts,
		DryRun:   flags.dryRun,
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
