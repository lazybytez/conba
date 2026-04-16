package backup

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/lazybytez/conba/internal/discovery"
)

// ErrTargetsFailed is returned by Run when one or more backup targets fail.
var ErrTargetsFailed = errors.New("backup targets failed")

// Func is the signature for a backup operation on a single path with tags.
type Func func(ctx context.Context, path string, tags []string) error

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
	failed := 0

	for _, target := range targets {
		if target.Mount.Source == "" {
			_, _ = fmt.Fprintf(
				out,
				"Skipped %s/%s: no source path\n",
				target.Container.Name,
				target.Mount.Name,
			)

			failed++

			continue
		}

		tags := BuildTags(target, hostname)

		err := backupFn(ctx, target.Mount.Source, tags)
		if err != nil {
			_, _ = fmt.Fprintf(
				out,
				"Failed %s/%s: %v\n",
				target.Container.Name,
				target.Mount.Name,
				err,
			)

			failed++

			continue
		}

		_, _ = fmt.Fprintf(out, "Backed up %s/%s\n", target.Container.Name, target.Mount.Name)

		succeeded++
	}

	_, _ = fmt.Fprintf(out, "Backup complete: %d succeeded, %d failed.\n", succeeded, failed)

	if failed > 0 {
		return fmt.Errorf("%d target(s) failed: %w", failed, ErrTargetsFailed)
	}

	return nil
}
