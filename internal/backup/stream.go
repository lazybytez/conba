package backup

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/runtime"
)

// StreamFunc is the signature for the restic-stdin sink. In production this is
// wired to (*restic.Client).BackupFromStdin: it reads the piped command output
// and captures it into a snapshot. The sink finalizes a snapshot only when it
// reads a clean EOF; if its context is cancelled before EOF it must abort with
// no snapshot (in production restic is terminated via the cancelled context).
type StreamFunc func(ctx context.Context, filename string, tags []string, stdin io.Reader) error

// RunStream executes a single stream backup for a labeled container. It runs
// the user's pre-backup command inside labeledContainer via the injected
// CommandExecer and pipes the command's stdout into the restic-stdin sink.
//
// The command always runs in labeledContainer. When spec.Filename is empty,
// the snapshot filename defaults to labeledContainer.
//
// The cmd handed to the executor is ["sh", "-c", <command>]; the user's
// command is interpreted only by the in-container shell. conba passes
// spec.Command verbatim and adds no outer shell.
//
// Exit-code safety: restic backup --stdin cannot detect a failed producer on
// its own -- a closed stdin is an ordinary EOF, so restic would finalize a
// snapshot of whatever partial bytes it received. To guarantee a non-zero
// command yields NO snapshot, the command's stdout is wired to restic's stdin
// through a real OS pipe (so restic reads the pipe directly, with no os/exec
// copy goroutine that would mask a producer error as EOF) and EOF is withheld
// until the command's exit status is known:
//
//   - command exits 0  -> close the write end (EOF) so restic finalizes;
//   - command exits !0  -> leave the write end open and cancel restic's
//     context, terminating it while it is still blocked waiting for EOF, so it
//     dies without committing.
//
// Because EOF is never delivered on the failure path until restic has already
// exited, there is no window in which restic could finalize a partial snapshot.
func RunStream(
	ctx context.Context,
	spec filter.Spec,
	labeledContainer string,
	hostname string,
	execer runtime.CommandExecer,
	streamFn StreamFunc,
) error {
	filename := resolveStreamFilename(spec, labeledContainer)
	cmd := []string{"sh", "-c", spec.Command}
	tags := BuildStreamTags(labeledContainer, hostname)

	pipeRead, pipeWrite, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("create stream pipe for %s: %w", labeledContainer, err)
	}

	defer func() { _ = pipeRead.Close() }()

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	execErrCh := make(chan error, 1)

	go func() {
		execErr := execer.Exec(streamCtx, labeledContainer, cmd, nil, pipeWrite)
		if execErr != nil {
			// Withhold EOF and terminate the sink: leave pipeWrite open so the
			// sink stays blocked on read, then cancel its context so restic is
			// killed before it can finalize a snapshot of the partial output.
			cancel()
		} else {
			// Deliver EOF so the sink finalizes the snapshot.
			_ = pipeWrite.Close()
		}

		execErrCh <- execErr
	}()

	sinkErr := streamFn(streamCtx, filename, tags, pipeRead)
	execErr := <-execErrCh

	// The sink has returned (it committed on success, or was terminated on
	// failure). Closing the write end now is cleanup only; on the failure path
	// the sink is already gone, so this can no longer trigger a commit.
	_ = pipeWrite.Close()

	return streamResult(labeledContainer, execErr, sinkErr)
}

// resolveStreamFilename returns the snapshot filename for a stream backup,
// defaulting to the labeled container name when the spec leaves it empty.
func resolveStreamFilename(spec filter.Spec, labeledContainer string) string {
	if spec.Filename == "" {
		return labeledContainer
	}

	return spec.Filename
}

// streamResult collapses the command and sink outcomes into a single error.
// The command is the root cause on the failure path, so its error takes
// precedence: a sink error there is the expected consequence of terminating
// restic. Both nil means the snapshot was committed.
func streamResult(labeledContainer string, execErr, sinkErr error) error {
	switch {
	case execErr != nil:
		return fmt.Errorf("run stream backup for %s: %w", labeledContainer, execErr)
	case sinkErr != nil:
		return fmt.Errorf("run stream backup for %s: %w", labeledContainer, sinkErr)
	default:
		return nil
	}
}
