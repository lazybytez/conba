package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lazybytez/conba/internal/build"
	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/report"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/spf13/cobra"
)

// resticDetectTimeout bounds the `restic version` probe so a hung restic
// cannot stall the version command.
const resticDetectTimeout = 5 * time.Second

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

	// version is config-free, so probe the default restic on PATH rather than
	// a configured binary path.
	ctx, cancel := context.WithTimeout(cmd.Context(), resticDetectTimeout)
	defer cancel()

	installed, detectErr := restic.DetectVersion(ctx, config.DefaultResticBinary)

	emitVersion(reporter, build.RecommendedResticVersion, installed, detectErr)

	return nil
}

// emitVersion writes the version event and, when warranted, a follow-up
// warning event: restic missing, or a restic major/minor mismatch.
func emitVersion(reporter report.Reporter, recommended, installed string, detectErr error) {
	reported := installed
	if detectErr != nil {
		reported = "not found"
	}

	reporter.Emit(report.Event{
		Level: report.LevelInfo,
		Name:  "version",
		Message: fmt.Sprintf(
			"conba %s (go: %s)\nrestic: %s (recommended %s)",
			build.ComputeVersionString(), build.GoVersion(), reported, recommended,
		),
		Fields: []report.Field{
			report.F("version", build.ComputeVersionString()),
			report.F("go", build.GoVersion()),
			report.F("restic_recommended", recommended),
			report.F("restic_installed", reported),
		},
	})

	if detectErr != nil {
		reporter.Emit(report.Event{
			Level:   report.LevelWarn,
			Style:   report.StyleWarning,
			Name:    "restic.not_found",
			Message: "WARNING: restic was not found on PATH; conba needs restic to run.",
			Fields:  []report.Field{report.F("recommended", recommended)},
		})

		return
	}

	match, ok := restic.VersionsCompatible(installed, recommended)
	if ok && !match {
		reporter.Emit(report.Event{
			Level: report.LevelWarn,
			Style: report.StyleWarning,
			Name:  "restic.version_mismatch",
			Message: fmt.Sprintf(
				"WARNING: installed restic %s differs from the recommended %s "+
					"(major/minor mismatch); behavior may differ.",
				installed, recommended,
			),
			Fields: []report.Field{
				report.F("installed", installed),
				report.F("recommended", recommended),
			},
		})
	}
}
