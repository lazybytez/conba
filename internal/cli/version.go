package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
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

// Labels standing in for the installed version in the text version line when
// the probe yielded none.
const (
	resticMissingLabel     = "not found"
	resticUnreadableLabel  = "unreadable"
	resticNotRunnableLabel = "not runnable"
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

	binary := resolveVersionProbeBinary(cmd)

	ctx, cancel := context.WithTimeout(cmd.Context(), resticDetectTimeout)
	defer cancel()

	installed, detectErr := restic.DetectVersion(ctx, binary)

	emitVersion(reporter, binary, build.RecommendedResticVersion, installed, detectErr)

	return nil
}

// resolveVersionProbeBinary returns the restic binary the version command
// probes: the configured one when the config loads, the default otherwise.
// Config problems are swallowed because version must report on a broken setup.
func resolveVersionProbeBinary(cmd *cobra.Command) string {
	cfg, err := config.Load(flagString(cmd.Flags(), "config"))
	if err != nil || cfg.Restic.Binary == "" {
		return config.DefaultResticBinary
	}

	return cfg.Restic.Binary
}

// emitVersion writes the version event and, when warranted, a follow-up
// warning event: restic missing, unrunnable, unreadable, or a major/minor
// mismatch.
// binary reaches the terminal quoted because it is an untrusted path from the
// config file; installed is rendered plainly, having already been validated
// where it was parsed out of the probed binary's stdout.
func emitVersion(
	reporter report.Reporter,
	binary, recommended, installed string,
	detectErr error,
) {
	reported := installed
	if detectErr != nil {
		reported = resticStatusLabel(detectErr)
	}

	reporter.Emit(report.Event{
		Level: report.LevelInfo,
		Name:  "version",
		Message: fmt.Sprintf(
			"conba %s (go: %s)\nrestic: %s at %q (recommended %s)",
			build.ComputeVersionString(), build.GoVersion(), reported, binary, recommended,
		),
		Fields: []report.Field{
			report.F("version", build.ComputeVersionString()),
			report.F("go", build.GoVersion()),
			report.F("restic_recommended", recommended),
			report.F("restic_installed", installed),
			report.F("restic_binary", binary),
		},
	})

	if detectErr != nil {
		reporter.Emit(resticProbeWarning(binary, recommended, detectErr))

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

// resticStatusLabel names why the probe returned no version: the binary is
// absent, it is there but could not be run, or it ran and its output did not
// parse.
func resticStatusLabel(detectErr error) string {
	switch {
	case errors.Is(detectErr, restic.ErrResticVersionParse):
		return resticUnreadableLabel
	case resticBinaryAbsent(detectErr):
		return resticMissingLabel
	default:
		return resticNotRunnableLabel
	}
}

// resticBinaryAbsent reports whether a probe failure means there is no binary
// at all: an unresolvable name, or a path with nothing behind it. Permission
// denials, non-zero exits, and timeouts all come from a binary that is there.
func resticBinaryAbsent(detectErr error) bool {
	return errors.Is(detectErr, exec.ErrNotFound) || errors.Is(detectErr, fs.ErrNotExist)
}

// resticProbeWarning builds the warning event for a failed probe: a missing
// binary, one that could not be run, or one whose output did not parse.
func resticProbeWarning(binary, recommended string, detectErr error) report.Event {
	name, message := resticProbeFailure(binary, detectErr)

	return report.Event{
		Level:   report.LevelWarn,
		Style:   report.StyleWarning,
		Name:    name,
		Message: message,
		Fields: []report.Field{
			report.F("binary", binary),
			report.F("recommended", recommended),
		},
	}
}

// resticProbeFailure returns the event name and warning text for a probe
// failure. The failure reason is quoted so a path echoed back by the operating
// system cannot break the warning across lines.
func resticProbeFailure(binary string, detectErr error) (string, string) {
	switch {
	case errors.Is(detectErr, restic.ErrResticVersionParse):
		return "restic.version_unreadable", fmt.Sprintf(
			"WARNING: restic at %q ran but its version output could not be read.",
			binary,
		)
	case resticBinaryAbsent(detectErr):
		return "restic.not_found", fmt.Sprintf(
			"WARNING: restic was not found (probed %q); conba needs restic to run.",
			binary,
		)
	default:
		return "restic.probe_failed", fmt.Sprintf(
			"WARNING: restic at %q was found but could not be run: %q.",
			binary, detectErr.Error(),
		)
	}
}
