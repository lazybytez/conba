package restore_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lazybytez/conba/internal/restore"
)

var (
	errRestore = errors.New("restore boom")
	errDump    = errors.New("dump boom")
	errExec    = errors.New("exec boom")
	errRunning = errors.New("running boom")
)

// restoreCall captures arguments to a stub RestoreFunc.
type restoreCall struct {
	snapshotID string
	targetPath string
	dryRun     bool
	called     bool
}

func stubRestoreFn(err error) (restore.RestoreFunc, *restoreCall) {
	captured := &restoreCall{
		snapshotID: "",
		targetPath: "",
		dryRun:     false,
		called:     false,
	}

	return func(_ context.Context, snapshotID, targetPath string, dryRun bool) error {
		captured.snapshotID = snapshotID
		captured.targetPath = targetPath
		captured.dryRun = dryRun
		captured.called = true

		return err
	}, captured
}

// dumpCall captures arguments and stdin behaviour of a stub DumpFunc.
type dumpCall struct {
	snapshotID string
	filename   string
	called     bool
	payload    []byte
	err        error
}

// stubDumpFn writes payload to the supplied stdout writer and then
// returns the configured error. The full payload is buffered before
// returning so the test can compare exactly what bytes were piped.
func stubDumpFn(payload []byte, err error) (restore.DumpFunc, *dumpCall) {
	captured := &dumpCall{
		snapshotID: "",
		filename:   "",
		called:     false,
		payload:    payload,
		err:        err,
	}

	return func(_ context.Context, snapshotID, filename string, stdout io.Writer) error {
		captured.snapshotID = snapshotID
		captured.filename = filename
		captured.called = true

		if len(payload) > 0 {
			_, writeErr := stdout.Write(payload)
			if writeErr != nil {
				return fmt.Errorf("stub write: %w", writeErr)
			}
		}

		return err
	}, captured
}

// fakeRuntime is an in-memory DockerRuntime double. Fields are populated
// in two phases: input fields (running, runningErr, execErr) are set by
// the test; output fields are populated by Exec() when it runs.
type fakeRuntime struct {
	// Inputs.
	running    bool
	runningErr error
	execErr    error

	// Outputs captured by Exec.
	execCalled     bool
	execName       string
	execArgv       []string
	execStdinBytes []byte
}

// newFakeRuntime constructs a fakeRuntime with all output fields zeroed
// so test sites do not have to spell the exhaustive struct literal each
// time.
func newFakeRuntime(running bool, runningErr, execErr error) *fakeRuntime {
	return &fakeRuntime{
		running:        running,
		runningErr:     runningErr,
		execErr:        execErr,
		execCalled:     false,
		execName:       "",
		execArgv:       nil,
		execStdinBytes: nil,
	}
}

func (f *fakeRuntime) ContainerRunning(_ context.Context, _ string) (bool, error) {
	return f.running, f.runningErr
}

func (f *fakeRuntime) Exec(_ context.Context, name string, argv []string, stdin io.Reader) error {
	f.execCalled = true
	f.execName = name
	f.execArgv = argv

	if stdin != nil {
		buf, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("read exec stdin: %w", err)
		}

		f.execStdinBytes = buf
	}

	return f.execErr
}

// --- RunVolume tests ---

func TestRunVolume_DryRunInvokesRestoreFnAndPrints(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	restoreFn, captured := stubRestoreFn(nil)

	opts := restore.Options{
		SnapshotID: "abc123",
		Filename:   "",
		Container:  "",
		TargetPath: "/restore/here",
		Command:    "",
		DryRun:     true,
		Force:      false,
		Out:        &buf,
	}

	err := restore.RunVolume(context.Background(), opts, restoreFn)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if !captured.called {
		t.Fatal("want restoreFn to be called in dry-run, was not")
	}

	if !captured.dryRun {
		t.Errorf("want dryRun=true passed to restoreFn, got false")
	}

	if captured.snapshotID != "abc123" {
		t.Errorf("snapshotID = %q, want %q", captured.snapshotID, "abc123")
	}

	if captured.targetPath != "/restore/here" {
		t.Errorf("targetPath = %q, want %q", captured.targetPath, "/restore/here")
	}

	want := "would restore snapshot abc123 to /restore/here\n"
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}

func TestRunVolume_DryRunPropagatesError(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	restoreFn, _ := stubRestoreFn(errRestore)

	opts := restore.Options{
		SnapshotID: "abc",
		Filename:   "",
		Container:  "",
		TargetPath: "/dest",
		Command:    "",
		DryRun:     true,
		Force:      false,
		Out:        &buf,
	}

	err := restore.RunVolume(context.Background(), opts, restoreFn)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(err, errRestore) {
		t.Errorf("want error wrapping errRestore, got %v", err)
	}
}

func TestRunVolume_LiveTargetEmpty_RestoreFnInvoked(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	restoreFn, captured := stubRestoreFn(nil)

	opts := restore.Options{
		SnapshotID: "snap1",
		Filename:   "",
		Container:  "",
		TargetPath: dir,
		Command:    "",
		DryRun:     false,
		Force:      false,
		Out:        io.Discard,
	}

	err := restore.RunVolume(context.Background(), opts, restoreFn)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if !captured.called {
		t.Fatal("want restoreFn to be called, was not")
	}

	if captured.dryRun {
		t.Errorf("want dryRun=false passed to restoreFn, got true")
	}

	if captured.targetPath != dir {
		t.Errorf("targetPath = %q, want %q", captured.targetPath, dir)
	}
}

func TestRunVolume_LiveTargetNonEmpty_NoForce_ErrDestinationNotEmpty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "preexisting.txt"), []byte("data"), 0o600)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	restoreFn, captured := stubRestoreFn(nil)

	opts := restore.Options{
		SnapshotID: "snap1",
		Filename:   "",
		Container:  "",
		TargetPath: dir,
		Command:    "",
		DryRun:     false,
		Force:      false,
		Out:        io.Discard,
	}

	gotErr := restore.RunVolume(context.Background(), opts, restoreFn)
	if gotErr == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(gotErr, restore.ErrDestinationNotEmpty) {
		t.Errorf("want error wrapping ErrDestinationNotEmpty, got %v", gotErr)
	}

	if captured.called {
		t.Error("want restoreFn NOT to be called when destination is non-empty without force")
	}
}

func TestRunVolume_LiveTargetNonEmpty_Force_RestoreFnInvoked(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	err := os.WriteFile(filepath.Join(dir, "preexisting.txt"), []byte("data"), 0o600)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	restoreFn, captured := stubRestoreFn(nil)

	opts := restore.Options{
		SnapshotID: "snap1",
		Filename:   "",
		Container:  "",
		TargetPath: dir,
		Command:    "",
		DryRun:     false,
		Force:      true,
		Out:        io.Discard,
	}

	gotErr := restore.RunVolume(context.Background(), opts, restoreFn)
	if gotErr != nil {
		t.Fatalf("want nil error with force=true, got %v", gotErr)
	}

	if !captured.called {
		t.Error("want restoreFn to be called with force=true on non-empty dest")
	}
}

func TestRunVolume_LiveTargetMissing_RestoreFnInvoked(t *testing.T) {
	t.Parallel()

	parent := t.TempDir()
	missing := filepath.Join(parent, "does-not-exist")

	restoreFn, captured := stubRestoreFn(nil)

	opts := restore.Options{
		SnapshotID: "snap1",
		Filename:   "",
		Container:  "",
		TargetPath: missing,
		Command:    "",
		DryRun:     false,
		Force:      false,
		Out:        io.Discard,
	}

	gotErr := restore.RunVolume(context.Background(), opts, restoreFn)
	if gotErr != nil {
		t.Fatalf("want nil error, got %v", gotErr)
	}

	if !captured.called {
		t.Error("want restoreFn to be called when destination does not exist")
	}
}

func TestRunVolume_RestoreFnError_Wrapped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	restoreFn, _ := stubRestoreFn(errRestore)

	opts := restore.Options{
		SnapshotID: "snap1",
		Filename:   "",
		Container:  "",
		TargetPath: dir,
		Command:    "",
		DryRun:     false,
		Force:      false,
		Out:        io.Discard,
	}

	gotErr := restore.RunVolume(context.Background(), opts, restoreFn)
	if gotErr == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(gotErr, errRestore) {
		t.Errorf("want error wrapping errRestore, got %v", gotErr)
	}
}

// --- RunStream tests ---

func TestRunStream_ContainerNotRunning_ErrContainerNotRunning(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntime(false, nil, nil)
	dumpFn, dumpCaptured := stubDumpFn([]byte("payload"), nil)

	var buf bytes.Buffer

	opts := restore.Options{
		SnapshotID: "abc",
		Filename:   "dump.sql",
		Container:  "mysql",
		TargetPath: "",
		Command:    "mysql",
		DryRun:     false,
		Force:      false,
		Out:        &buf,
	}

	err := restore.RunStream(context.Background(), opts, dumpFn, runtime)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(err, restore.ErrContainerNotRunning) {
		t.Errorf("want error wrapping ErrContainerNotRunning, got %v", err)
	}

	if dumpCaptured.called {
		t.Error("dumpFn should NOT be called when container not running")
	}

	if runtime.execCalled {
		t.Error("runtime.Exec should NOT be called when container not running")
	}
}

func TestRunStream_ContainerRunningError_Wrapped(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntime(false, errRunning, nil)
	dumpFn, _ := stubDumpFn([]byte{}, nil)

	opts := restore.Options{
		SnapshotID: "abc",
		Filename:   "dump.sql",
		Container:  "mysql",
		TargetPath: "",
		Command:    "mysql",
		DryRun:     false,
		Force:      false,
		Out:        io.Discard,
	}

	err := restore.RunStream(context.Background(), opts, dumpFn, runtime)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(err, errRunning) {
		t.Errorf("want error wrapping errRunning, got %v", err)
	}
}

func TestRunStream_DryRun_PrintsAndDoesNotInvoke(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntime(true, nil, nil)
	dumpFn, dumpCaptured := stubDumpFn([]byte("payload"), nil)

	var buf bytes.Buffer

	opts := restore.Options{
		SnapshotID: "abc",
		Filename:   "dump.sql",
		Container:  "mysql",
		TargetPath: "",
		Command:    "mysql -uroot",
		DryRun:     true,
		Force:      false,
		Out:        &buf,
	}

	err := restore.RunStream(context.Background(), opts, dumpFn, runtime)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if dumpCaptured.called {
		t.Error("dumpFn should NOT be called in dry-run")
	}

	if runtime.execCalled {
		t.Error("runtime.Exec should NOT be called in dry-run")
	}

	want := "would restore snapshot abc by piping dump.sql into mysql -uroot in container mysql\n"
	if buf.String() != want {
		t.Errorf("output = %q, want %q", buf.String(), want)
	}
}

func TestRunStream_LivePath_BuildsArgvAndPipesPayload(t *testing.T) {
	t.Parallel()

	payload := []byte("INSERT INTO t VALUES (1);\n")

	runtime := newFakeRuntime(true, nil, nil)
	dumpFn, dumpCaptured := stubDumpFn(payload, nil)

	opts := restore.Options{
		SnapshotID: "snap-xyz",
		Filename:   "dump.sql",
		Container:  "mysql",
		TargetPath: "",
		Command:    "mysql -uroot",
		DryRun:     false,
		Force:      false,
		Out:        io.Discard,
	}

	err := restore.RunStream(context.Background(), opts, dumpFn, runtime)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if !dumpCaptured.called {
		t.Fatal("want dumpFn to be called on live path")
	}

	if dumpCaptured.snapshotID != "snap-xyz" {
		t.Errorf("dump snapshotID = %q, want %q", dumpCaptured.snapshotID, "snap-xyz")
	}

	if dumpCaptured.filename != "dump.sql" {
		t.Errorf("dump filename = %q, want %q", dumpCaptured.filename, "dump.sql")
	}

	if !runtime.execCalled {
		t.Fatal("want runtime.Exec to be called on live path")
	}

	if runtime.execName != "mysql" {
		t.Errorf("exec name = %q, want %q", runtime.execName, "mysql")
	}

	wantArgv := []string{"docker", "exec", "-i", "mysql", "sh", "-c", "mysql -uroot"}
	if !reflect.DeepEqual(runtime.execArgv, wantArgv) {
		t.Errorf("argv = %v, want %v", runtime.execArgv, wantArgv)
	}

	if !bytes.Equal(runtime.execStdinBytes, payload) {
		t.Errorf("piped bytes = %q, want %q", runtime.execStdinBytes, payload)
	}
}

func TestRunStream_DumpFnError_WrappedWithDumpPhase(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntime(true, nil, nil)
	dumpFn, _ := stubDumpFn([]byte("partial"), errDump)

	opts := restore.Options{
		SnapshotID: "snap1",
		Filename:   "dump.sql",
		Container:  "mysql",
		TargetPath: "",
		Command:    "mysql",
		DryRun:     false,
		Force:      false,
		Out:        io.Discard,
	}

	err := restore.RunStream(context.Background(), opts, dumpFn, runtime)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(err, errDump) {
		t.Errorf("want error wrapping errDump, got %v", err)
	}

	if !contains(err.Error(), "dump") {
		t.Errorf("want error to mention 'dump' phase, got %q", err.Error())
	}
}

func TestRunStream_ExecError_WrappedWithExecPhase(t *testing.T) {
	t.Parallel()

	runtime := newFakeRuntime(true, nil, errExec)
	dumpFn, _ := stubDumpFn([]byte("payload"), nil)

	opts := restore.Options{
		SnapshotID: "snap1",
		Filename:   "dump.sql",
		Container:  "mysql",
		TargetPath: "",
		Command:    "mysql",
		DryRun:     false,
		Force:      false,
		Out:        io.Discard,
	}

	err := restore.RunStream(context.Background(), opts, dumpFn, runtime)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(err, errExec) {
		t.Errorf("want error wrapping errExec, got %v", err)
	}

	if !contains(err.Error(), "exec") {
		t.Errorf("want error to mention 'exec' phase, got %q", err.Error())
	}
}

// contains is a tiny strings.Contains shim that avoids pulling strings into
// the imports just for one assertion. Test-only.
func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}

	return false
}
