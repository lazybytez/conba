// Package cli assembles the conba command tree and provides the Execute
// entry point called by main.
package cli

import (
	"fmt"
	"os"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/lazybytez/conba/internal/report"
	"github.com/spf13/cobra"
)

// NewRootCommand creates the root conba command with all subcommands registered.
func NewRootCommand() *cobra.Command {
	var (
		cfgFile      string
		outputFormat string
		noColor      bool
	)

	cmd := &cobra.Command{
		Use:   "conba",
		Short: "A simple restic-based container volume backup tool",
		Long: "Conba automates restic backups for container volumes.\n" +
			"It reads a declarative config and runs backup and restore operations.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			mode, color := report.Resolve(outputFormat, cfg.Output.Format, os.Stdout)
			if noColor {
				color = false
			}

			logger, err := logging.New(cfg.Logging.Level, mode == report.ModeJSON, color)
			if err != nil {
				return fmt.Errorf("init logger: %w", err)
			}

			ctx := config.WithConfig(cmd.Context(), cfg)
			ctx = logging.WithLogger(ctx, logger)
			ctx = report.WithReporter(ctx, report.New(mode, cmd.OutOrStdout(), color))
			cmd.SetContext(ctx)

			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "path to config file")
	cmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "",
		"output format: text or json (default: auto-detect by terminal)")
	cmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colored output")
	cmd.AddCommand(NewVersionCommand())
	cmd.AddCommand(NewInitCommand())
	cmd.AddCommand(NewInspectCommand())
	cmd.AddCommand(NewStatusCommand())
	cmd.AddCommand(NewUnlockCommand())
	cmd.AddCommand(NewBackupCommand())
	cmd.AddCommand(NewSnapshotsCommand())
	cmd.AddCommand(NewForgetCommand())
	cmd.AddCommand(NewRunCommand())
	cmd.AddCommand(NewRestoreCommand())
	cmd.AddCommand(NewDiffCommand())
	cmd.AddCommand(NewVerifyCommand())

	return cmd
}

// Execute builds the root command and runs it. It is the single entry point
// called by main.
func Execute() error {
	err := NewRootCommand().Execute()
	if err != nil {
		return fmt.Errorf("executing command: %w", err)
	}

	return nil
}
