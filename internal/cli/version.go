package cli

import (
	"fmt"

	"github.com/lazybytez/conba/internal/build"
	"github.com/spf13/cobra"
)

// NewVersionCommand creates the version subcommand.
func NewVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return nil
		},
		RunE: runVersion,
	}
}

func runVersion(cmd *cobra.Command, _ []string) error {
	_, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"conba %s (go: %s, restic: %s)\n",
		build.ComputeVersionString(),
		build.GoVersion(),
		build.ResticVersion,
	)
	if err != nil {
		return fmt.Errorf("writing version output: %w", err)
	}

	return nil
}
