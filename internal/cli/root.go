// Package cli assembles the conba command tree and provides the Execute
// entry point called by main.
package cli

import (
	"fmt"

	"github.com/lazybytez/conba/internal/config"
	"github.com/spf13/cobra"
)

// NewRootCommand creates the root conba command with all subcommands registered.
func NewRootCommand() *cobra.Command {
	var cfgFile string

	cmd := &cobra.Command{
		Use:   "conba",
		Short: "A simple restic-based container volume backup tool",
		Long: "Conba automates restic backups for container volumes.\n" +
			"It reads a declarative config and runs backup and restore operations.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			_, err := config.Load(cfgFile)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "path to config file")
	cmd.AddCommand(NewVersionCommand())

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
