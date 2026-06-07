package backup

import (
	"context"
	"errors"
	"fmt"

	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/report"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/lazybytez/conba/internal/runtime"
)

// ErrTargetsFailed is returned by Run when one or more backup targets fail.
var ErrTargetsFailed = errors.New("backup targets failed")

// RunError reports how a backup cycle ended, carrying per-outcome counts so
// callers can distinguish a partial failure from a total one. It satisfies
// errors.Is(err, ErrTargetsFailed).
type RunError struct {
	Succeeded int
	Skipped   int
	Failed    int
}

// Error implements error.
func (e *RunError) Error() string {
	total := e.Succeeded + e.Skipped + e.Failed

	return fmt.Sprintf("%d of %d target(s) failed", e.Failed, total)
}

// Is reports that a RunError matches the ErrTargetsFailed sentinel.
func (e *RunError) Is(target error) bool {
	return target == ErrTargetsFailed
}

// Func is the signature for a backup operation on a single path with tags.
type Func func(ctx context.Context, path string, tags []string) error

// Options bundles the dependencies and inputs required by Run.
type Options struct {
	BackupFn         Func
	StreamFn         StreamFunc
	Execer           runtime.CommandExecer
	PreBackupEnabled bool
	Hostname         string
}

// targetOutcome classifies the result of backing up a single target.
type targetOutcome int

const (
	outcomeSucceeded targetOutcome = iota
	outcomeSkipped
	outcomeFailed
)

// counts aggregates per-target outcomes across a backup cycle.
type counts struct {
	succeeded int
	skipped   int
	failed    int
}

func (c *counts) add(o targetOutcome) {
	switch o {
	case outcomeSucceeded:
		c.succeeded++
	case outcomeSkipped:
		c.skipped++
	case outcomeFailed:
		c.failed++
	}
}

// Run executes backups for all targets sequentially, emitting progress and a
// summary through reporter. Targets are grouped by container so that the
// optional pre-backup stream sub-operation runs at most once per labeled
// container per cycle.
//
// When opts.PreBackupEnabled is false, container labels are ignored and every
// target is backed up via opts.BackupFn as a plain volume backup. When true,
// containers carrying the conba.pre-backup.* labels dispatch through
// opts.StreamFn; volume sub-operations for those containers are skipped
// (replace mode) or run alongside the stream (alongside mode).
//
// Returns a *RunError (matchable via errors.Is(err, ErrTargetsFailed)) if any
// target failed.
func Run(
	ctx context.Context,
	targets []discovery.Target,
	opts Options,
	reporter report.Reporter,
) error {
	if len(targets) == 0 {
		return nil
	}

	totals := counts{succeeded: 0, skipped: 0, failed: 0}

	for _, group := range groupByContainer(targets) {
		runGroup(ctx, group, opts, reporter, &totals)
	}

	reporter.Emit(report.Event{
		Level: report.LevelInfo,
		Name:  "backup.summary",
		Message: fmt.Sprintf(
			"Backup complete: %d succeeded, %d skipped, %d failed.",
			totals.succeeded, totals.skipped, totals.failed,
		),
		Fields: []report.Field{
			report.F("succeeded", totals.succeeded),
			report.F("skipped", totals.skipped),
			report.F("failed", totals.failed),
		},
	})

	if totals.failed > 0 {
		return &RunError{
			Succeeded: totals.succeeded,
			Skipped:   totals.skipped,
			Failed:    totals.failed,
		}
	}

	return nil
}

// groupByContainer partitions targets into stable per-container groups,
// preserving discovery order within and across groups.
func groupByContainer(targets []discovery.Target) [][]discovery.Target {
	var (
		groups []([]discovery.Target)
		index  = map[string]int{}
	)

	for _, target := range targets {
		key := target.Container.ID
		if pos, ok := index[key]; ok {
			groups[pos] = append(groups[pos], target)

			continue
		}

		index[key] = len(groups)
		groups = append(groups, []discovery.Target{target})
	}

	return groups
}

// runGroup processes one container's targets, branching on whether the
// container has a pre-backup spec and whether the feature is enabled.
func runGroup(
	ctx context.Context,
	group []discovery.Target,
	opts Options,
	reporter report.Reporter,
	totals *counts,
) {
	if !opts.PreBackupEnabled {
		runVolumeOnly(ctx, group, opts.BackupFn, opts.Hostname, reporter, totals)

		return
	}

	first := group[0]

	spec, hasSpec, err := filter.PreBackup(first)
	if err != nil {
		reporter.Emit(report.Event{
			Level: report.LevelError,
			Style: report.StyleError,
			Name:  "backup.stream",
			Message: fmt.Sprintf(
				"Failed %s stream: invalid pre-backup labels: %v", first.Container.Name, err,
			),
			Fields: []report.Field{
				report.F("container", first.Container.Name),
				report.F("outcome", "failed"),
				report.F("error", err.Error()),
			},
		})

		totals.add(outcomeFailed)

		return
	}

	if !hasSpec {
		runVolumeOnly(ctx, group, opts.BackupFn, opts.Hostname, reporter, totals)

		return
	}

	runStreamOnce(ctx, spec, first.Container.Name, opts, reporter, totals)
	runVolumesForLabeledGroup(ctx, group, opts.BackupFn, spec, opts.Hostname, reporter, totals)
}

// runVolumeOnly performs a per-target volume backup loop without consulting
// pre-backup labels.
func runVolumeOnly(
	ctx context.Context,
	group []discovery.Target,
	backupFn Func,
	hostname string,
	reporter report.Reporter,
	totals *counts,
) {
	for _, target := range group {
		totals.add(runTarget(ctx, target, backupFn, hostname, reporter))
	}
}

// runStreamOnce dispatches the single stream sub-operation for a labeled
// container and reports its outcome under a synthetic "<container> stream"
// identity.
func runStreamOnce(
	ctx context.Context,
	spec filter.Spec,
	containerName string,
	opts Options,
	reporter report.Reporter,
	totals *counts,
) {
	err := RunStream(ctx, spec, containerName, opts.Hostname, opts.Execer, opts.StreamFn)
	if err != nil {
		reporter.Emit(report.Event{
			Level:   report.LevelError,
			Style:   report.StyleError,
			Name:    "backup.stream",
			Message: fmt.Sprintf("Failed %s stream: %v", containerName, err),
			Fields: []report.Field{
				report.F("container", containerName),
				report.F("outcome", "failed"),
				report.F("error", err.Error()),
			},
		})

		totals.add(outcomeFailed)

		return
	}

	reporter.Emit(report.Event{
		Level:   report.LevelInfo,
		Style:   report.StyleSuccess,
		Name:    "backup.stream",
		Message: fmt.Sprintf("Backed up %s stream", containerName),
		Fields: []report.Field{
			report.F("container", containerName),
			report.F("outcome", "success"),
		},
	})

	totals.add(outcomeSucceeded)
}

// runVolumesForLabeledGroup handles the volume side for a group whose
// container carries a pre-backup spec. In replace mode the volume backups
// are skipped; in alongside mode they run regardless of stream outcome.
func runVolumesForLabeledGroup(
	ctx context.Context,
	group []discovery.Target,
	backupFn Func,
	spec filter.Spec,
	hostname string,
	reporter report.Reporter,
	totals *counts,
) {
	if spec.Mode == filter.ModeReplace {
		for _, target := range group {
			reporter.Emit(report.Event{
				Level: report.LevelInfo,
				Name:  "backup.target",
				Message: fmt.Sprintf(
					"Skipped %s/%s: replaced by pre-backup stream",
					target.Container.Name, target.Mount.Name,
				),
				Fields: []report.Field{
					report.F("container", target.Container.Name),
					report.F("volume", target.Mount.Name),
					report.F("outcome", "skipped"),
					report.F("reason", "replaced by pre-backup stream"),
				},
			})

			totals.add(outcomeSkipped)
		}

		return
	}

	for _, target := range group {
		totals.add(runTarget(ctx, target, backupFn, hostname, reporter))
	}
}

// runTarget backs up a single target and returns the outcome.
func runTarget(
	ctx context.Context,
	target discovery.Target,
	backupFn Func,
	hostname string,
	reporter report.Reporter,
) targetOutcome {
	if target.Mount.Source == "" {
		reporter.Emit(report.Event{
			Level: report.LevelWarn,
			Style: report.StyleWarning,
			Name:  "backup.target",
			Message: fmt.Sprintf(
				"Skipped %s/%s: no source path", target.Container.Name, target.Mount.Name,
			),
			Fields: []report.Field{
				report.F("container", target.Container.Name),
				report.F("volume", target.Mount.Name),
				report.F("outcome", "skipped"),
				report.F("reason", "no source path"),
			},
		})

		return outcomeSkipped
	}

	tags := BuildVolumeTags(target, hostname)

	err := backupFn(ctx, target.Mount.Source, tags)
	if err != nil {
		return reportTargetError(reporter, target, err)
	}

	reporter.Emit(report.Event{
		Level:   report.LevelInfo,
		Style:   report.StyleSuccess,
		Name:    "backup.target",
		Message: fmt.Sprintf("Backed up %s/%s", target.Container.Name, target.Mount.Name),
		Fields: []report.Field{
			report.F("container", target.Container.Name),
			report.F("volume", target.Mount.Name),
			report.F("outcome", "success"),
		},
	})

	return outcomeSucceeded
}

// reportTargetError emits the warn (source unreadable) or error (failure)
// event for a failed backupFn call and returns the matching outcome.
func reportTargetError(
	reporter report.Reporter,
	target discovery.Target,
	err error,
) targetOutcome {
	if errors.Is(err, restic.ErrSourceUnreadable) {
		reporter.Emit(report.Event{
			Level: report.LevelWarn,
			Style: report.StyleWarning,
			Name:  "backup.target",
			Message: fmt.Sprintf(
				"WARN: skipping %s/%s: source unreadable (%v)",
				target.Container.Name, target.Mount.Destination, err,
			),
			Fields: []report.Field{
				report.F("container", target.Container.Name),
				report.F("volume", target.Mount.Name),
				report.F("outcome", "skipped"),
				report.F("reason", "source unreadable"),
			},
		})

		return outcomeSkipped
	}

	reporter.Emit(report.Event{
		Level:   report.LevelError,
		Style:   report.StyleError,
		Name:    "backup.target",
		Message: fmt.Sprintf("Failed %s/%s: %v", target.Container.Name, target.Mount.Name, err),
		Fields: []report.Field{
			report.F("container", target.Container.Name),
			report.F("volume", target.Mount.Name),
			report.F("outcome", "failed"),
			report.F("error", err.Error()),
		},
	})

	return outcomeFailed
}
