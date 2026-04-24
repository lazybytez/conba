package backup

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/restic"
)

// ErrTargetsFailed is returned by Run when one or more backup targets fail.
var ErrTargetsFailed = errors.New("backup targets failed")

// Func is the signature for a backup operation on a single path with tags.
type Func func(ctx context.Context, path string, tags []string) error

// targetOutcome classifies the result of backing up a single target.
type targetOutcome int

const (
	outcomeSucceeded targetOutcome = iota
	outcomeSkipped
	outcomeFailed
)

// Run executes backups for all targets sequentially, writing progress to out.
// It returns an error if any target fails.
func Run(
	ctx context.Context,
	targets []discovery.Target,
	backupFn Func,
	hostname string,
	out io.Writer,
) error {
	if len(targets) == 0 {
		return nil
	}

	succeeded := 0
	skipped := 0
	failed := 0

	for _, target := range targets {
		switch runTarget(ctx, target, backupFn, hostname, out) {
		case outcomeSucceeded:
			succeeded++
		case outcomeSkipped:
			skipped++
		case outcomeFailed:
			failed++
		}
	}

	_, _ = fmt.Fprintf(
		out,
		"Backup complete: %d succeeded, %d skipped, %d failed.\n",
		succeeded,
		skipped,
		failed,
	)

	if failed > 0 {
		return fmt.Errorf("%d target(s) failed: %w", failed, ErrTargetsFailed)
	}

	return nil
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

		return outcomeFailed
	}

	tags := BuildTags(target, hostname)

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
