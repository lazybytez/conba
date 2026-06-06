package backup

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/lazybytez/conba/internal/runtime"
)

// ErrTargetsFailed is returned by Run when one or more backup targets fail.
var ErrTargetsFailed = errors.New("backup targets failed")

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

// Run executes backups for all targets sequentially, writing progress to out.
// Targets are grouped by container so that the optional pre-backup stream
// sub-operation runs at most once per labeled container per cycle.
//
// When opts.PreBackupEnabled is false, container labels are ignored and every
// target is backed up via opts.BackupFn as a plain volume backup (existing
// behaviour). When true, containers carrying the conba.pre-backup.* labels
// dispatch through opts.StreamFn; volume sub-operations for those containers
// are skipped (replace mode) or run alongside the stream (alongside mode).
//
// Returns a wrapped ErrTargetsFailed if any target failed.
func Run(
	ctx context.Context,
	targets []discovery.Target,
	opts Options,
	out io.Writer,
) error {
	if len(targets) == 0 {
		return nil
	}

	totals := counts{succeeded: 0, skipped: 0, failed: 0}

	for _, group := range groupByContainer(targets) {
		runGroup(ctx, group, opts, out, &totals)
	}

	_, _ = fmt.Fprintf(
		out,
		"Backup complete: %d succeeded, %d skipped, %d failed.\n",
		totals.succeeded,
		totals.skipped,
		totals.failed,
	)

	if totals.failed > 0 {
		return fmt.Errorf("%d target(s) failed: %w", totals.failed, ErrTargetsFailed)
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
	out io.Writer,
	totals *counts,
) {
	if !opts.PreBackupEnabled {
		runVolumeOnly(ctx, group, opts.BackupFn, opts.Hostname, out, totals)

		return
	}

	first := group[0]

	spec, hasSpec, err := filter.PreBackup(first)
	if err != nil {
		_, _ = fmt.Fprintf(
			out,
			"Failed %s stream: invalid pre-backup labels: %v\n",
			first.Container.Name,
			err,
		)

		totals.add(outcomeFailed)

		return
	}

	if !hasSpec {
		runVolumeOnly(ctx, group, opts.BackupFn, opts.Hostname, out, totals)

		return
	}

	runStreamOnce(
		ctx,
		spec,
		first.Container.Name,
		opts.Hostname,
		opts.Execer,
		opts.StreamFn,
		out,
		totals,
	)

	runVolumesForLabeledGroup(ctx, group, opts.BackupFn, spec, opts.Hostname, out, totals)
}

// runVolumeOnly performs a per-target volume backup loop without consulting
// pre-backup labels.
func runVolumeOnly(
	ctx context.Context,
	group []discovery.Target,
	backupFn Func,
	hostname string,
	out io.Writer,
	totals *counts,
) {
	for _, target := range group {
		totals.add(runTarget(ctx, target, backupFn, hostname, out))
	}
}

// runStreamOnce dispatches the single stream sub-operation for a labeled
// container and reports its outcome under a synthetic "<container> stream"
// identity.
func runStreamOnce(
	ctx context.Context,
	spec filter.Spec,
	containerName string,
	hostname string,
	execer runtime.CommandExecer,
	streamFn StreamFunc,
	out io.Writer,
	totals *counts,
) {
	err := RunStream(ctx, spec, containerName, hostname, execer, streamFn)
	if err != nil {
		_, _ = fmt.Fprintf(out, "Failed %s stream: %v\n", containerName, err)

		totals.add(outcomeFailed)

		return
	}

	_, _ = fmt.Fprintf(out, "Backed up %s stream\n", containerName)

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
	out io.Writer,
	totals *counts,
) {
	if spec.Mode == filter.ModeReplace {
		for _, target := range group {
			_, _ = fmt.Fprintf(
				out,
				"Skipped %s/%s: replaced by pre-backup stream\n",
				target.Container.Name,
				target.Mount.Name,
			)

			totals.add(outcomeSkipped)
		}

		return
	}

	for _, target := range group {
		totals.add(runTarget(ctx, target, backupFn, hostname, out))
	}
}

// runTarget backs up a single target and returns the outcome.
func runTarget(
	ctx context.Context,
	target discovery.Target,
	backupFn Func,
	hostname string,
	out io.Writer,
) targetOutcome {
	if target.Mount.Source == "" {
		_, _ = fmt.Fprintf(
			out,
			"Skipped %s/%s: no source path\n",
			target.Container.Name,
			target.Mount.Name,
		)

		return outcomeSkipped
	}

	tags := BuildVolumeTags(target, hostname)

	err := backupFn(ctx, target.Mount.Source, tags)
	if err != nil {
		if errors.Is(err, restic.ErrSourceUnreadable) {
			_, _ = fmt.Fprintf(
				out,
				"WARN: skipping %s/%s: source unreadable (%v)\n",
				target.Container.Name,
				target.Mount.Destination,
				err,
			)

			return outcomeSkipped
		}

		_, _ = fmt.Fprintf(
			out,
			"Failed %s/%s: %v\n",
			target.Container.Name,
			target.Mount.Name,
			err,
		)

		return outcomeFailed
	}

	_, _ = fmt.Fprintf(out, "Backed up %s/%s\n", target.Container.Name, target.Mount.Name)

	return outcomeSucceeded
}
