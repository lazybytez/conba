package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/report"
)

func formatTags(tags []string) string {
	return strings.Join(tags, ", ")
}

// emitDryRun renders the backup dry-run plan as structured events: one
// backup.plan per planned action plus a backup.plan.summary with the volume
// count. Mirrors printDryRun's logic for json output.
func emitDryRun(
	reporter report.Reporter,
	targets []discovery.Target,
	preBackupEnabled bool,
) {
	volumeCount := 0

	for _, group := range groupByContainer(targets) {
		volumeCount += emitDryRunGroup(reporter, group, preBackupEnabled)
	}

	reporter.Emit(report.Event{
		Level:   report.LevelInfo,
		Name:    "backup.plan.summary",
		Message: fmt.Sprintf("%d volume(s) would be backed up.", volumeCount),
		Fields:  []report.Field{report.F("volumes", volumeCount)},
	})
}

func emitDryRunGroup(
	reporter report.Reporter,
	group []discovery.Target,
	preBackupEnabled bool,
) int {
	first := group[0]

	spec, hasSpec, err := filter.PreBackup(first)
	if !preBackupEnabled || err != nil || !hasSpec {
		return emitDryRunVolumes(reporter, group)
	}

	reporter.Emit(report.Event{
		Level: report.LevelInfo,
		Name:  "backup.plan",
		Fields: []report.Field{
			report.F("container", first.Container.Name),
			report.F("action", "run_command"),
			report.F("command", spec.Command),
			report.F("mode", string(spec.Mode)),
		},
	})

	if spec.Mode != filter.ModeReplace {
		return emitDryRunVolumes(reporter, group)
	}

	for _, target := range group {
		reporter.Emit(report.Event{
			Level: report.LevelInfo,
			Name:  "backup.plan",
			Fields: []report.Field{
				report.F("container", target.Container.Name),
				report.F("volume", target.Mount.Name),
				report.F("action", "skip"),
				report.F("reason", "replaced by pre-backup stream"),
			},
		})
	}

	return 0
}

func emitDryRunVolumes(reporter report.Reporter, group []discovery.Target) int {
	for _, target := range group {
		reporter.Emit(report.Event{
			Level: report.LevelInfo,
			Name:  "backup.plan",
			Fields: []report.Field{
				report.F("container", target.Container.Name),
				report.F("volume", target.Mount.Name),
				report.F("action", "backup"),
			},
		})
	}

	return len(group)
}

// printDryRun renders the dry-run plan for a backup cycle. When
// preBackupEnabled is true, a container carrying valid pre-backup labels
// emits one "would run: <cmd> in <container>" line and either replaces the
// volume listing with a "would skip" line (replace mode) or keeps the
// per-mount listing alongside the run line (alongside mode). When
// preBackupEnabled is false, pre-backup labels are ignored and every
// container renders the per-mount listing only, matching what the actual
// backup would do with the feature disabled.
func printDryRun(out io.Writer, targets []discovery.Target, preBackupEnabled bool) error {
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("get hostname: %w", err)
	}

	var volumeCount int

	for _, group := range groupByContainer(targets) {
		count, groupErr := writeDryRunGroup(out, group, hostname, preBackupEnabled)
		if groupErr != nil {
			return groupErr
		}

		volumeCount += count
	}

	_, err = fmt.Fprintf(out, "%d volume(s) would be backed up.\n", volumeCount)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

// writeDryRunGroup renders one container group's dry-run output, returning
// the number of volume targets that would be backed up (i.e. excluding
// replaced mounts). When preBackupEnabled is false, or the container has no
// pre-backup labels, the group renders via the legacy listing; labeled
// containers (feature enabled) emit a "would run:" line followed by
// per-mount skip/listing depending on mode.
func writeDryRunGroup(
	out io.Writer,
	group []discovery.Target,
	hostname string,
	preBackupEnabled bool,
) (int, error) {
	if !preBackupEnabled {
		return writeDryRunLegacyGroup(out, group, hostname)
	}

	first := group[0]

	spec, hasSpec, err := filter.PreBackup(first)
	if err != nil || !hasSpec {
		return writeDryRunLegacyGroup(out, group, hostname)
	}

	_, runErr := fmt.Fprintf(
		out,
		"would run: %s in %s\n",
		spec.Command,
		first.Container.Name,
	)
	if runErr != nil {
		return 0, fmt.Errorf("writing output: %w", runErr)
	}

	if spec.Mode == filter.ModeReplace {
		writeErr := writeDryRunReplaceSkips(out, group)
		if writeErr != nil {
			return 0, writeErr
		}

		return 0, nil
	}

	// Alongside mode keeps the legacy volume listing for each mount.
	count, listErr := writeDryRunLegacyGroup(out, group, hostname)
	if listErr != nil {
		return 0, listErr
	}

	return count, nil
}

// writeDryRunReplaceSkips emits one "would skip" line per mount in a
// replace-mode group and a trailing blank line for spacing.
func writeDryRunReplaceSkips(out io.Writer, group []discovery.Target) error {
	for _, target := range group {
		_, err := fmt.Fprintf(
			out,
			"would skip: %s/%s -- replaced by pre-backup stream\n",
			target.Container.Name,
			target.Mount.Name,
		)
		if err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
	}

	_, err := fmt.Fprintln(out)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

// writeDryRunLegacyGroup renders the legacy per-target dry-run listing for
// one container group, returning the number of mounts written.
func writeDryRunLegacyGroup(
	out io.Writer,
	group []discovery.Target,
	hostname string,
) (int, error) {
	for _, target := range group {
		err := writeDryRunTarget(out, target, hostname)
		if err != nil {
			return 0, err
		}
	}

	return len(group), nil
}

// writeDryRunTarget renders one target's legacy dry-run block: header,
// mount line, tag list, and trailing blank line.
func writeDryRunTarget(out io.Writer, target discovery.Target, hostname string) error {
	tags := backup.BuildVolumeTags(target, hostname)

	_, err := fmt.Fprintf(out, "%s (%s)\n",
		target.Container.Name,
		shortID(target.Container.ID))
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	_, err = fmt.Fprintf(out, "  %s  %s → %s\n",
		target.Mount.Type,
		target.Mount.Name,
		target.Mount.Source)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	_, err = fmt.Fprintf(out, "  tags: %s\n",
		formatTags(tags))
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	_, err = fmt.Fprintln(out)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}
