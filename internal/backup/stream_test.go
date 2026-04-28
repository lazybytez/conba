package backup_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/filter"
)

var errStream = errors.New("stream backup failed")

type streamCall struct {
	filename string
	tags     []string
	args     []string
}

func stubStreamFn(err error) (backup.StreamFunc, *streamCall) {
	captured := &streamCall{
		filename: "",
		tags:     nil,
		args:     nil,
	}

	return func(_ context.Context, filename string, tags []string, args []string) error {
		captured.filename = filename
		captured.tags = tags
		captured.args = args

		return err
	}, captured
}

func TestRunStream_DefaultExecTarget(t *testing.T) {
	t.Parallel()

	spec := filter.Spec{
		Command:        "mysqldump --all-databases",
		Mode:           filter.ModeReplace,
		Container:      "",
		Filename:       "",
		RestoreCommand: "",
	}

	fn, captured := stubStreamFn(nil)

	err := backup.RunStream(context.Background(), spec, "mysql", "host01", fn)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	wantArgs := []string{"docker", "exec", "mysql", "sh", "-c", "mysqldump --all-databases"}
	if !reflect.DeepEqual(captured.args, wantArgs) {
		t.Errorf("args = %v, want %v", captured.args, wantArgs)
	}
}

func TestRunStream_OverrideExecTarget(t *testing.T) {
	t.Parallel()

	spec := filter.Spec{
		Command:        "pg_dumpall",
		Mode:           filter.ModeReplace,
		Container:      "pg-sidecar",
		Filename:       "",
		RestoreCommand: "",
	}

	fn, captured := stubStreamFn(nil)

	err := backup.RunStream(context.Background(), spec, "postgres", "host01", fn)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	wantArgs := []string{"docker", "exec", "pg-sidecar", "sh", "-c", "pg_dumpall"}
	if !reflect.DeepEqual(captured.args, wantArgs) {
		t.Errorf("args = %v, want %v", captured.args, wantArgs)
	}
}

func TestRunStream_DefaultFilename(t *testing.T) {
	t.Parallel()

	spec := filter.Spec{
		Command:        "echo hello",
		Mode:           filter.ModeReplace,
		Container:      "",
		Filename:       "",
		RestoreCommand: "",
	}

	fn, captured := stubStreamFn(nil)

	err := backup.RunStream(context.Background(), spec, "myapp", "host01", fn)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if captured.filename != "myapp" {
		t.Errorf("filename = %q, want %q", captured.filename, "myapp")
	}
}

func TestRunStream_CustomFilename(t *testing.T) {
	t.Parallel()

	spec := filter.Spec{
		Command:        "mysqldump",
		Mode:           filter.ModeReplace,
		Container:      "",
		Filename:       "dump.sql",
		RestoreCommand: "",
	}

	fn, captured := stubStreamFn(nil)

	err := backup.RunStream(context.Background(), spec, "mysql", "host01", fn)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if captured.filename != "dump.sql" {
		t.Errorf("filename = %q, want %q", captured.filename, "dump.sql")
	}
}

func TestRunStream_TagsMatchHelper(t *testing.T) {
	t.Parallel()

	spec := filter.Spec{
		Command:        "echo hi",
		Mode:           filter.ModeReplace,
		Container:      "",
		Filename:       "",
		RestoreCommand: "",
	}

	fn, captured := stubStreamFn(nil)

	err := backup.RunStream(context.Background(), spec, "mysql", "host01", fn)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	want := backup.BuildStreamTags("mysql", "host01")
	if !reflect.DeepEqual(captured.tags, want) {
		t.Errorf("tags = %v, want %v", captured.tags, want)
	}
}

func TestRunStream_PropagatesError(t *testing.T) {
	t.Parallel()

	spec := filter.Spec{
		Command:        "false",
		Mode:           filter.ModeReplace,
		Container:      "",
		Filename:       "",
		RestoreCommand: "",
	}

	fn, _ := stubStreamFn(errStream)

	err := backup.RunStream(context.Background(), spec, "myapp", "host01", fn)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(err, errStream) {
		t.Errorf("want error wrapping errStream, got %v", err)
	}
}
