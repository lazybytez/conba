package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/forget"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/lazybytez/conba/internal/runtime/docker"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

// runFlags holds the CLI-provided flags for the run command.
type runFlags struct {
	dryRun   bool
	allHosts bool
	noForget bool
}

// runRequest bundles the dependencies and inputs shared by the
// run command's phase helpers.
type runRequest struct {
	cmd      *cobra.Command
	cfg      *config.Config
	logger   *zap.Logger
	client   *restic.Client
	hostname string
	flags    runFlags
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
	req, err := buildRunRequest(cmd)
	if err != nil {
		return err
	}

	err = runInitPhase(req)
	if err != nil {
		return err
	}

	targets, runtimeClient, cleanup, err := discoverFiltered(req.cmd.Context(), req.cfg, req.logger)
	if err != nil {
		return err
	}

	// Keep the docker client open across the whole cycle: runBackupPhase wires
	// it as the Execer so backup.Run can run pre-backup commands via the SDK.
	defer cleanup()

	if len(targets) == 0 {
		_, writeErr := fmt.Fprintln(req.cmd.OutOrStdout(),
			"No volumes discovered; nothing to back up or forget.")
		if writeErr != nil {
			return fmt.Errorf("writing output: %w", writeErr)
		}

		return nil
	}

	err = runBackupPhase(req, runtimeClient, targets)
	if err != nil {
		return err
	}

	if req.flags.noForget {
		return nil
	}

	return runForgetPhase(req, targets)
}

func buildRunRequest(cmd *cobra.Command) (*runRequest, error) {
	ctx := cmd.Context()
	cfg := config.FromContext(ctx)
	logger := logging.FromContext(ctx)

	if cfg == nil {
		return nil, errMissingConfig
	}

	flags := readRunFlags(cmd.Flags())

	client, err := restic.New(cfg.Restic, logger)
	if err != nil {
		return nil, fmt.Errorf("create restic client: %w", err)
	}

	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("get hostname: %w", err)
	}

	return &runRequest{
		cmd:      cmd,
		cfg:      cfg,
		logger:   logger,
		client:   client,
		hostname: hostname,
		flags:    flags,
	}, nil
}

func readRunFlags(flags *pflag.FlagSet) runFlags {
	return runFlags{
		dryRun:   flagBool(flags, "dry-run"),
		allHosts: flagBool(flags, "all-hosts"),
		noForget: flagBool(flags, "no-forget"),
	}
}

func runInitPhase(req *runRequest) error {
	out := req.cmd.OutOrStdout()

	err := writePhaseHeader(out, "init")
	if err != nil {
		return err
	}

	err = req.client.Init(req.cmd.Context())
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
	req *runRequest,
	runtimeClient *docker.Client,
	targets []discovery.Target,
) error {
	out := req.cmd.OutOrStdout()

	err := writePhaseHeader(out, "backup")
	if err != nil {
		return err
	}

	if req.flags.dryRun {
		return printDryRun(out, targets, req.cfg.PreBackupCommands.Enabled)
	}

	opts := buildBackupOptions(req.client, runtimeClient, req.cfg, req.hostname)

	err = backup.Run(req.cmd.Context(), targets, opts, out)
	if err != nil {
		return fmt.Errorf("run backup: %w", err)
	}

	return nil
}

func runForgetPhase(req *runRequest, targets []discovery.Target) error {
	out := req.cmd.OutOrStdout()

	err := writePhaseHeader(out, "forget")
	if err != nil {
		return err
	}

	opts := buildForgetOptions(req.hostname, req.flags.allHosts, req.flags.dryRun, true)

	err = forget.Run(req.cmd.Context(), targets, req.client.Forget, req.cfg.Retention, opts, out)
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
