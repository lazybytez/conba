package cli

import (
	"fmt"
	"os"

	"github.com/lazybytez/conba/internal/build"
	"github.com/lazybytez/conba/internal/report"
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
	// version skips config loading (and thus the root reporter), so it
	// resolves the output mode from the inherited --output/--no-color flags
	// directly. The flags are absent when RunE is invoked on a detached
	// command; flagString then yields "" and the mode falls back to auto.
	mode, color := report.Resolve(flagString(cmd.Flags(), "output"), "auto", os.Stdout)
	if flagBool(cmd.Flags(), "no-color") {
		color = false
	}

	reporter := report.New(mode, cmd.OutOrStdout(), color)
	reporter.Emit(report.Event{
		Level: report.LevelInfo,
		Name:  "version",
		Message: fmt.Sprintf(
			"conba %s (go: %s, restic: %s)",
			build.ComputeVersionString(), build.GoVersion(), build.ResticVersion,
		),
		Fields: []report.Field{
			report.F("version", build.ComputeVersionString()),
			report.F("go", build.GoVersion()),
			report.F("restic", build.ResticVersion),
		},
	})

	return nil
}
