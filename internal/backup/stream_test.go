package backup_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/filter"
)

var (
	errStream = errors.New("stream backup failed")
	errExec   = errors.New("exec failed")
)

// fakeExecer captures the Exec arguments and either writes a payload to the
// supplied stdout writer or returns an injected error.
type fakeExecer struct {
	container string
	cmd       []string
	payload   []byte
	err       error
}

func (f *fakeExecer) Exec(
	_ context.Context,
	container string,
	cmd []string,
	_ io.Reader,
	stdout io.Writer,
) error {
	f.container = container
	f.cmd = cmd

	// Model a real command: any stdout produced before failure is written to
	// the pipe (restic may read it), then the non-zero exit is reported.
	if len(f.payload) > 0 {
		_, err := stdout.Write(f.payload)
		if err != nil {
			return fmt.Errorf("fake exec write: %w", err)
		}
	}

	if f.err != nil {
		return f.err
	}

	return nil
}

// fakeSink models the restic-stdin sink. It drains the reader in a goroutine
// and finalizes (completed) only on a clean EOF. If its context is cancelled
// before EOF -- modelling conba terminating restic on a failed command -- it
// records killed and finalizes nothing.
type fakeSink struct {
	filename  string
	tags      []string
	received  []byte
	readErr   error
	completed bool
	killed    bool
}

func (s *fakeSink) fn() backup.StreamFunc {
	return func(ctx context.Context, filename string, tags []string, stdin io.Reader) error {
		s.filename = filename
		s.tags = tags

		type readResult struct {
			data []byte
			err  error
		}

		done := make(chan readResult, 1)

		go func() {
			data, err := io.ReadAll(stdin)
			done <- readResult{data: data, err: err}
		}()

		select {
		case <-ctx.Done():
			s.killed = true

			return fmt.Errorf("fake sink terminated: %w", ctx.Err())
		case result := <-done:
			s.received = result.data

			if result.err != nil {
				s.readErr = result.err

				return fmt.Errorf("fake sink read: %w", result.err)
			}

			s.completed = true

			return nil
		}
	}
}

func newSink() *fakeSink {
	return &fakeSink{
		filename:  "",
		tags:      nil,
		received:  nil,
		readErr:   nil,
		completed: false,
		killed:    false,
	}
}

func TestRunStream_CmdIsShDashC(t *testing.T) {
	t.Parallel()

	spec := filter.Spec{
		Command:  "mysqldump --all-databases",
		Mode:     filter.ModeReplace,
		Filename: "",
	}

	execer := &fakeExecer{container: "", cmd: nil, payload: []byte("dump"), err: nil}
	sink := newSink()

	err := backup.RunStream(context.Background(), spec, "mysql", "host01", execer, sink.fn())
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	wantCmd := []string{"sh", "-c", "mysqldump --all-databases"}
	if !reflect.DeepEqual(execer.cmd, wantCmd) {
		t.Errorf("cmd = %v, want %v", execer.cmd, wantCmd)
	}

	if execer.container != "mysql" {
		t.Errorf("container = %q, want %q", execer.container, "mysql")
	}
}

func TestRunStream_SuccessPayloadFlows(t *testing.T) {
	t.Parallel()

	spec := filter.Spec{
		Command:  "echo hello",
		Mode:     filter.ModeReplace,
		Filename: "",
	}

	payload := []byte("the full backup payload")
	execer := &fakeExecer{container: "", cmd: nil, payload: payload, err: nil}
	sink := newSink()

	err := backup.RunStream(context.Background(), spec, "myapp", "host01", execer, sink.fn())
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if !sink.completed {
		t.Error("want sink to complete successfully, got incomplete")
	}

	if !bytes.Equal(sink.received, payload) {
		t.Errorf("sink received %q, want %q", sink.received, payload)
	}

	if sink.readErr != nil {
		t.Errorf("want nil read error on success, got %v", sink.readErr)
	}
}

// TestRunStream_FailingExecAborts is the unit-level guard for the integrity
// rule: when the command exits non-zero, RunStream must terminate the sink
// (cancel its context) so it finalizes NO snapshot, and surface the exec error.
func TestRunStream_FailingExecAborts(t *testing.T) {
	t.Parallel()

	spec := filter.Spec{
		Command:  "echo partial; exit 1",
		Mode:     filter.ModeReplace,
		Filename: "",
	}

	// Partial output is written, then the command fails.
	execer := &fakeExecer{container: "", cmd: nil, payload: []byte("partial"), err: errExec}
	sink := newSink()

	err := backup.RunStream(context.Background(), spec, "myapp", "host01", execer, sink.fn())
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(err, errExec) {
		t.Errorf("want error wrapping errExec, got %v", err)
	}

	if sink.completed {
		t.Error("want sink NOT to finalize a snapshot when the command fails")
	}

	if !sink.killed {
		t.Error("want sink to be terminated (context cancelled) when the command fails")
	}
}

func TestRunStream_DefaultFilename(t *testing.T) {
	t.Parallel()

	spec := filter.Spec{
		Command:  "echo hello",
		Mode:     filter.ModeReplace,
		Filename: "",
	}

	execer := &fakeExecer{container: "", cmd: nil, payload: nil, err: nil}
	sink := newSink()

	err := backup.RunStream(context.Background(), spec, "myapp", "host01", execer, sink.fn())
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if sink.filename != "myapp" {
		t.Errorf("filename = %q, want %q", sink.filename, "myapp")
	}
}

func TestRunStream_CustomFilename(t *testing.T) {
	t.Parallel()

	spec := filter.Spec{
		Command:  "mysqldump",
		Mode:     filter.ModeReplace,
		Filename: "dump.sql",
	}

	execer := &fakeExecer{container: "", cmd: nil, payload: nil, err: nil}
	sink := newSink()

	err := backup.RunStream(context.Background(), spec, "mysql", "host01", execer, sink.fn())
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if sink.filename != "dump.sql" {
		t.Errorf("filename = %q, want %q", sink.filename, "dump.sql")
	}
}

func TestRunStream_TagsMatchHelper(t *testing.T) {
	t.Parallel()

	spec := filter.Spec{
		Command:  "echo hi",
		Mode:     filter.ModeReplace,
		Filename: "",
	}

	execer := &fakeExecer{container: "", cmd: nil, payload: nil, err: nil}
	sink := newSink()

	err := backup.RunStream(context.Background(), spec, "mysql", "host01", execer, sink.fn())
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	want := backup.BuildStreamTags("mysql", "host01")
	if !reflect.DeepEqual(sink.tags, want) {
		t.Errorf("tags = %v, want %v", sink.tags, want)
	}
}

func TestRunStream_PropagatesSinkError(t *testing.T) {
	t.Parallel()

	spec := filter.Spec{
		Command:  "echo hi",
		Mode:     filter.ModeReplace,
		Filename: "",
	}

	execer := &fakeExecer{container: "", cmd: nil, payload: []byte("data"), err: nil}

	sinkFn := func(_ context.Context, _ string, _ []string, stdin io.Reader) error {
		_, _ = io.Copy(io.Discard, stdin)

		return errStream
	}

	err := backup.RunStream(context.Background(), spec, "myapp", "host01", execer, sinkFn)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(err, errStream) {
		t.Errorf("want error wrapping errStream, got %v", err)
	}
}
