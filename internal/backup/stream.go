package backup

import (
	"context"
	"fmt"

	"github.com/lazybytez/conba/internal/filter"
)

// StreamFunc is the signature for a backup-from-command operation.
// In production, this is wired to (*restic.Client).BackupFromCommand.
type StreamFunc func(ctx context.Context, filename string, tags []string, args []string) error

// RunStream executes a single stream backup for a labeled container by
// dispatching the user's pre-backup command through `docker exec` and
// capturing its stdout via the supplied StreamFunc.
//
// Resolution rules:
//   - When spec.Container is empty, the command runs in labeledContainer.
//   - When spec.Filename is empty, the snapshot filename is labeledContainer.
//
// The argv passed to fn is array-form ["docker", "exec", <execContainer>,
// "sh", "-c", <command>]. The user's command is interpreted only by the
// in-container shell; conba passes spec.Command verbatim.
func RunStream(
	ctx context.Context,
	spec filter.Spec,
	labeledContainer string,
	hostname string,
	streamFn StreamFunc,
) error {
	execContainer := spec.Container
	if execContainer == "" {
		execContainer = labeledContainer
	}

	filename := spec.Filename
	if filename == "" {
		filename = labeledContainer
	}

	args := []string{"docker", "exec", execContainer, "sh", "-c", spec.Command}
	tags := BuildStreamTags(labeledContainer, hostname)

	err := streamFn(ctx, filename, tags, args)
	if err != nil {
		return fmt.Errorf("run stream backup for %s: %w", labeledContainer, err)
	}

	return nil
}
