// Package restore orchestrates restic-backed restore operations from the
// CLI layer. Two modes are supported: ModeVolume materialises a volume
// snapshot back to a host path, ModeStream pipes a stream snapshot's
// stored file into a docker exec command running inside a target
// container. The package follows the same dependency-injection pattern
// as internal/backup: production wires function-typed callbacks to real
// restic.Client methods while tests inject fakes.
package restore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// Mode identifies which kind of snapshot is being restored.
type Mode int

// Mode values.
const (
	// ModeVolume restores a snapshot of a host directory back to a host path.
	ModeVolume Mode = iota
	// ModeStream pipes a stream snapshot into a command inside a container.
	ModeStream
)

// RestoreFunc is the production-wires-to-Client.Restore signature. It
// performs a restic restore of snapshotID into targetPath and honours
// dryRun (no files are written when true). The verbose name pairs with
// DumpFunc to make the two callback types unambiguous at the call site.
//
//nolint:revive // Paired naming with DumpFunc; the stutter is intentional.
type RestoreFunc func(
	ctx context.Context,
	snapshotID, targetPath string,
	dryRun bool,
) error

// DumpFunc is the production-wires-to-Client.Dump signature. It writes
// the contents of filename inside snapshotID to stdout. The orchestrator
// pipes that stdout into another process via io.Pipe.
type DumpFunc func(
	ctx context.Context,
	snapshotID, filename string,
	stdout io.Writer,
) error

// DockerRuntime narrows what RunStream needs from the docker runtime.
// Production wires this to internal/runtime/docker via a thin adapter
// constructed by the CLI layer; tests inject a fake.
type DockerRuntime interface {
	// ContainerRunning reports whether the named container is currently
	// running. It returns (false, nil) when the container exists but is
	// not running, and (false, err) on lookup failure.
	ContainerRunning(ctx context.Context, name string) (bool, error)

	// Exec runs argv inside the named container with stdin attached. It
	// blocks until the command exits and returns a non-nil error on a
	// non-zero exit status.
	Exec(ctx context.Context, name string, argv []string, stdin io.Reader) error
}

// Options bundles the resolved inputs the orchestrator needs. The CLI
// layer resolves SnapshotID upstream via restic.Client.Snapshots and
// restic.ResolveSnapshot before populating this struct.
type Options struct {
	// SnapshotID is the restic snapshot to restore (already resolved).
	SnapshotID string
	// Filename is the stream snapshot's stored filename. It mirrors the
	// --stdin-filename used at backup time. Stream mode only.
	Filename string
	// Container is the docker exec target for stream mode.
	Container string
	// TargetPath is the host directory to restore into. Volume mode only.
	TargetPath string
	// Command is the sh -c content piped through docker exec. Stream mode only.
	Command string
	// DryRun reports actions without performing them.
	DryRun bool
	// Force overrides the non-empty destination guard. Volume mode only.
	Force bool
	// Out receives human-readable progress lines.
	Out io.Writer
}

// Sentinel errors. The CLI maps these via errors.Is to specific
// exit codes and messages.
var (
	// ErrDestinationNotEmpty is returned by RunVolume when the target
	// path contains entries and Force is false.
	ErrDestinationNotEmpty = errors.New("destination path is not empty")

	// ErrContainerNotRunning is returned by RunStream when the target
	// container is not currently running.
	ErrContainerNotRunning = errors.New("container is not running")
)

// RunVolume executes a volume restore. In dry-run mode it prints a
// single descriptive line and invokes restoreFn with dryRun=true so
// restic reports its intended actions. In live mode it first checks
// that the target directory is empty (unless opts.Force is true) and
// then invokes restoreFn with dryRun=false.
func RunVolume(
	ctx context.Context,
	opts Options,
	restoreFn RestoreFunc,
) error {
	if opts.DryRun {
		_, _ = fmt.Fprintf(
			opts.Out,
			"would restore snapshot %s to %s\n",
			opts.SnapshotID,
			opts.TargetPath,
		)

		err := restoreFn(ctx, opts.SnapshotID, opts.TargetPath, true)
		if err != nil {
			return fmt.Errorf("dry-run volume restore: %w", err)
		}

		return nil
	}

	if !opts.Force {
		err := requireEmptyDir(opts.TargetPath)
		if err != nil {
			return err
		}
	}

	err := restoreFn(ctx, opts.SnapshotID, opts.TargetPath, false)
	if err != nil {
		return fmt.Errorf("volume restore: %w", err)
	}

	return nil
}

// requireEmptyDir returns ErrDestinationNotEmpty when path exists and
// contains entries. A missing path or read failure due to absence is
// treated as "empty" -- restic creates the destination on demand.
func requireEmptyDir(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("read destination %s: %w", path, err)
	}

	if len(entries) > 0 {
		return fmt.Errorf("%w: %s", ErrDestinationNotEmpty, path)
	}

	return nil
}

// RunStream pipes the named file from a stream snapshot into a docker
// exec command running inside opts.Container. The argv form mirrors the
// backup-side pattern: ["docker", "exec", "-i", <container>, "sh",
// "-c", <command>]. The user's command is interpreted only by the
// in-container shell; conba never joins it into a host shell string.
func RunStream(
	ctx context.Context,
	opts Options,
	dumpFn DumpFunc,
	docker DockerRuntime,
) error {
	running, err := docker.ContainerRunning(ctx, opts.Container)
	if err != nil {
		return fmt.Errorf("check container running: %w", err)
	}

	if !running {
		return fmt.Errorf("%w: %s", ErrContainerNotRunning, opts.Container)
	}

	argv := []string{"docker", "exec", "-i", opts.Container, "sh", "-c", opts.Command}

	if opts.DryRun {
		_, _ = fmt.Fprintf(
			opts.Out,
			"would restore snapshot %s by piping %s into %s in container %s\n",
			opts.SnapshotID,
			opts.Filename,
			opts.Command,
			opts.Container,
		)

		return nil
	}

	return runStreamPipe(ctx, opts, argv, dumpFn, docker)
}

// runStreamPipe wires dumpFn -> io.Pipe -> docker.Exec(stdin) and
// collects errors from both phases, returning a wrapped error that
// names the failing phase.
func runStreamPipe(
	ctx context.Context,
	opts Options,
	argv []string,
	dumpFn DumpFunc,
	docker DockerRuntime,
) error {
	pipeReader, pipeWriter := io.Pipe()

	dumpErrCh := make(chan error, 1)

	go func() {
		dumpErr := dumpFn(ctx, opts.SnapshotID, opts.Filename, pipeWriter)
		// Closing the writer end with the error (or nil) propagates EOF
		// or the dump failure to the reader side, allowing docker.Exec
		// to observe end-of-stream.
		_ = pipeWriter.CloseWithError(dumpErr)
		dumpErrCh <- dumpErr
	}()

	execErr := docker.Exec(ctx, opts.Container, argv, pipeReader)
	// Drain the pipe reader so the dump goroutine is not blocked on a
	// full pipe buffer when Exec returns early on its own error.
	_ = pipeReader.Close()

	dumpErr := <-dumpErrCh
	if dumpErr != nil {
		return fmt.Errorf("dump phase: %w", dumpErr)
	}

	if execErr != nil {
		return fmt.Errorf("exec phase: %w", execErr)
	}

	return nil
}
