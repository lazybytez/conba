package docker

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/lazybytez/conba/internal/runtime"
)

// stderrTailCap bounds the stderr captured from an exec. A chatty failing
// command must not be able to grow the error message without limit, so only
// the most recent bytes are retained for diagnostics.
const stderrTailCap = 4 * 1024

// errExecNonZeroExit is the sentinel returned when an in-container command
// exits non-zero. Callers match it with errors.Is to abort dependent work.
var errExecNonZeroExit = errors.New("exec command exited non-zero")

// Compile-time assertion that *Client satisfies the capability interface.
var _ runtime.CommandExecer = (*Client)(nil)

// Exec runs cmd inside the named container using the Docker Engine exec API.
// cmd is passed to the daemon as a discrete argument vector, so no outer shell
// interprets it and no argument injection is possible at this layer. stdin and
// stdout are attached only when non-nil; stderr is always captured into a
// bounded tail used for diagnostics. A non-zero exit code yields a non-nil
// error wrapping errExecNonZeroExit.
func (c *Client) Exec(
	ctx context.Context,
	container string,
	cmd []string,
	stdin io.Reader,
	stdout io.Writer,
) error {
	execID, err := c.createExec(ctx, container, cmd, stdin, stdout)
	if err != nil {
		return err
	}

	stderrTail, err := c.runExec(ctx, execID, stdin, stdout)
	if err != nil {
		return err
	}

	inspect, err := c.docker.ContainerExecInspect(ctx, execID)
	if err != nil {
		return fmt.Errorf("inspect exec in container %q: %w", container, err)
	}

	return execExitError(inspect.ExitCode, stderrTail)
}

// createExec issues the exec-create call and returns the resulting exec ID.
func (c *Client) createExec(
	ctx context.Context,
	containerName string,
	cmd []string,
	stdin io.Reader,
	stdout io.Writer,
) (string, error) {
	resp, err := c.docker.ContainerExecCreate(ctx, containerName, container.ExecOptions{
		User:         "",
		Privileged:   false,
		Tty:          false,
		ConsoleSize:  nil,
		AttachStdin:  stdin != nil,
		AttachStderr: true,
		AttachStdout: stdout != nil,
		DetachKeys:   "",
		Env:          nil,
		WorkingDir:   "",
		Cmd:          cmd,
		Detach:       false,
	})
	if err != nil {
		return "", fmt.Errorf("create exec in container %q: %w", containerName, err)
	}

	return resp.ID, nil
}

// runExec attaches to the exec stream, pumps stdin when provided, demultiplexes
// stdout/stderr, and returns the bounded stderr tail.
func (c *Client) runExec(
	ctx context.Context,
	execID string,
	stdin io.Reader,
	stdout io.Writer,
) ([]byte, error) {
	resp, err := c.docker.ContainerExecAttach(ctx, execID, container.ExecAttachOptions{
		Detach:      false,
		Tty:         false,
		ConsoleSize: nil,
	})
	if err != nil {
		return nil, fmt.Errorf("attach to exec: %w", err)
	}
	defer resp.Close()

	stdinErrCh := pumpStdin(execStdin{attach: &resp}, stdin)

	stdoutTarget := io.Discard
	if stdout != nil {
		stdoutTarget = stdout
	}

	stderrTail := newTailWriter(stderrTailCap)

	_, err = stdcopy.StdCopy(stdoutTarget, stderrTail, resp.Reader)
	if err != nil {
		return nil, fmt.Errorf("read exec output stream: %w", err)
	}

	err = <-stdinErrCh
	if err != nil {
		return nil, err
	}

	return stderrTail.Bytes(), nil
}

// stdinWriter is the writable half of an attached exec session that pumpStdin
// needs: a writer for the command's stdin and a half-close to signal
// end-of-input. Defining it locally keeps the helper testable and the
// dependency on the Docker SDK narrow.
type stdinWriter interface {
	io.Writer
	CloseWrite() error
}

// execStdin adapts a Docker exec attachment to stdinWriter. ContainerExecAttach
// returns a types.HijackedResponse: the daemon "hijacks" the HTTP connection
// and hands back a raw bidirectional stream. conba writes the command's stdin
// to that stream's Conn and half-closes the write side with CloseWrite, leaving
// stdout/stderr open so any remaining output still drains.
type execStdin struct {
	attach *types.HijackedResponse
}

func (s execStdin) Write(p []byte) (int, error) {
	n, err := s.attach.Conn.Write(p)
	if err != nil {
		return n, fmt.Errorf("write to exec stdin: %w", err)
	}

	return n, nil
}

func (s execStdin) CloseWrite() error {
	err := s.attach.CloseWrite()
	if err != nil {
		return fmt.Errorf("half-close exec stdin: %w", err)
	}

	return nil
}

// pumpStdin copies stdin into the exec stream in a goroutine and reports any
// copy error on the returned channel. When stdin is nil the channel yields a
// single nil. The goroutine has a clear owner: the channel is read exactly
// once by runExec after StdCopy returns.
func pumpStdin(stream stdinWriter, stdin io.Reader) <-chan error {
	errCh := make(chan error, 1)

	if stdin == nil {
		errCh <- nil

		return errCh
	}

	go func() {
		_, copyErr := io.Copy(stream, stdin)

		closeErr := stream.CloseWrite()

		if copyErr != nil {
			errCh <- fmt.Errorf("copy stdin to exec: %w", copyErr)

			return
		}

		if closeErr != nil {
			errCh <- fmt.Errorf("close exec stdin: %w", closeErr)

			return
		}

		errCh <- nil
	}()

	return errCh
}

// execExitError maps an exec exit code to an error. Exit 0 returns nil; any
// other code returns a non-nil error wrapping errExecNonZeroExit and including
// the bounded stderr tail for diagnostics.
func execExitError(exitCode int, stderrTail []byte) error {
	if exitCode == 0 {
		return nil
	}

	if len(stderrTail) == 0 {
		return fmt.Errorf("%w: exit code %d", errExecNonZeroExit, exitCode)
	}

	return fmt.Errorf(
		"%w: exit code %d: %s",
		errExecNonZeroExit,
		exitCode,
		stderrTail,
	)
}

// tailWriter is an io.Writer that retains only the last cap bytes written to
// it. It is used to capture a bounded stderr tail so a failing command that
// floods stderr cannot exhaust memory.
type tailWriter struct {
	buf []byte
	cap int
}

// newTailWriter creates a tailWriter that keeps at most capBytes bytes.
func newTailWriter(capBytes int) *tailWriter {
	return &tailWriter{
		buf: make([]byte, 0, capBytes),
		cap: capBytes,
	}
}

// Write appends data, discarding all but the trailing cap bytes. It reports
// the full length of data as written so it never signals a short write.
func (t *tailWriter) Write(data []byte) (int, error) {
	total := len(data)

	if t.cap <= 0 {
		return total, nil
	}

	if len(data) >= t.cap {
		t.buf = append(t.buf[:0], data[len(data)-t.cap:]...)

		return total, nil
	}

	t.buf = append(t.buf, data...)
	if len(t.buf) > t.cap {
		t.buf = append(t.buf[:0], t.buf[len(t.buf)-t.cap:]...)
	}

	return total, nil
}

// Bytes returns the currently retained tail. The returned slice aliases the
// internal buffer and must not be mutated by the caller.
func (t *tailWriter) Bytes() []byte {
	return t.buf
}
