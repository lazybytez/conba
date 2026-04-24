package backup_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/backup"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/lazybytez/conba/internal/runtime"
)

var errBackup = errors.New("backup failed")

func stubBackupFn(errs ...error) (backup.Func, *[]string) {
	var paths []string

	callIndex := 0

	return func(_ context.Context, path string, _ []string) error {
		paths = append(paths, path)

		if callIndex < len(errs) {
			err := errs[callIndex]
			callIndex++

			return err
		}

		callIndex++

		return nil
	}, &paths
}

func makeTarget(name, source, mountName string) discovery.Target {
	return discovery.Target{
		Container: runtime.ContainerInfo{
			ID:     "c-" + name,
			Name:   name,
			Labels: nil,
			Mounts: nil,
		},
		Mount: runtime.MountInfo{
			Type:        runtime.MountTypeVolume,
			Name:        mountName,
			Source:      source,
			Destination: "/" + mountName,
			ReadOnly:    false,
		},
	}
}

func TestRun_AllSucceed(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("app", "/src/app-data", "data"),
		makeTarget("db", "/src/db-data", "pgdata"),
	}

	fn, _ := stubBackupFn(nil, nil)

	var buf bytes.Buffer

	err := backup.Run(context.Background(), targets, fn, "host1", &buf)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	output := buf.String()

	if strings.Count(output, "Backed up") != 2 {
		t.Errorf("want 2 'Backed up' lines, got output:\n%s", output)
	}

	if !strings.Contains(output, "2 succeeded, 0 skipped, 0 failed") {
		t.Errorf("want summary '2 succeeded, 0 skipped, 0 failed', got output:\n%s", output)
	}
}

func TestRun_AllFail(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("app", "/src/app-data", "data"),
		makeTarget("db", "/src/db-data", "pgdata"),
	}

	fn, _ := stubBackupFn(errBackup, errBackup)

	var buf bytes.Buffer

	err := backup.Run(context.Background(), targets, fn, "host1", &buf)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(err, backup.ErrTargetsFailed) {
		t.Errorf("want error wrapping ErrTargetsFailed, got %v", err)
	}

	output := buf.String()

	if strings.Count(output, "Failed") != 2 {
		t.Errorf("want 2 'Failed' lines, got output:\n%s", output)
	}

	if !strings.Contains(output, "0 succeeded, 0 skipped, 2 failed") {
		t.Errorf("want summary '0 succeeded, 0 skipped, 2 failed', got output:\n%s", output)
	}
}

func TestRun_PartialFailure(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("app", "/src/app-data", "data"),
		makeTarget("db", "/src/db-data", "pgdata"),
	}

	fn, _ := stubBackupFn(nil, errBackup)

	var buf bytes.Buffer

	err := backup.Run(context.Background(), targets, fn, "host1", &buf)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	output := buf.String()

	if !strings.Contains(output, "Backed up app/data") {
		t.Errorf("want 'Backed up app/data' line, got output:\n%s", output)
	}

	if !strings.Contains(output, "Failed db/pgdata") {
		t.Errorf("want 'Failed db/pgdata' line, got output:\n%s", output)
	}

	if !strings.Contains(output, "1 succeeded, 0 skipped, 1 failed") {
		t.Errorf("want summary '1 succeeded, 0 skipped, 1 failed', got output:\n%s", output)
	}
}

func TestRun_Empty(t *testing.T) {
	t.Parallel()

	fn, _ := stubBackupFn()

	var buf bytes.Buffer

	err := backup.Run(context.Background(), nil, fn, "host1", &buf)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("want no output, got %q", buf.String())
	}
}

func TestRun_UsesSourcePath(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("app", "/host/path/to/data", "data"),
	}

	fn, paths := stubBackupFn(nil)

	var buf bytes.Buffer

	err := backup.Run(context.Background(), targets, fn, "host1", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(*paths) != 1 {
		t.Fatalf("want 1 path, got %d", len(*paths))
	}

	if (*paths)[0] != "/host/path/to/data" {
		t.Errorf("want path %q, got %q", "/host/path/to/data", (*paths)[0])
	}
}

func TestRun_PassesBuildTags(t *testing.T) {
	t.Parallel()

	target := makeTarget("myapp", "/src/myapp-vol", "vol")
	targets := []discovery.Target{target}

	var capturedTags []string

	backupFn := func(_ context.Context, _ string, tags []string) error {
		capturedTags = tags

		return nil
	}

	var buf bytes.Buffer

	err := backup.Run(context.Background(), targets, backupFn, "testhost", &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := backup.BuildTags(target, "testhost")

	if len(capturedTags) != len(expected) {
		t.Fatalf("want %d tags, got %d", len(expected), len(capturedTags))
	}

	for i := range expected {
		if capturedTags[i] != expected[i] {
			t.Errorf("tag[%d] = %q, want %q", i, capturedTags[i], expected[i])
		}
	}
}

func TestRun_EmptySourceSkipped(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("app", "", "data"),
	}

	fn, paths := stubBackupFn()

	var buf bytes.Buffer

	err := backup.Run(context.Background(), targets, fn, "host1", &buf)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if len(*paths) != 0 {
		t.Errorf("want 0 backupFn calls, got %d", len(*paths))
	}

	output := buf.String()

	if !strings.Contains(output, "Skipped app/data: no source path") {
		t.Errorf("want skip message, got output:\n%s", output)
	}

	if !strings.Contains(output, "0 succeeded, 0 skipped, 1 failed") {
		t.Errorf("want summary '0 succeeded, 0 skipped, 1 failed', got output:\n%s", output)
	}
}

func TestRun_SourceUnreadableIsSkipped(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("app", "/src/app-data", "data"),
	}

	wrapped := fmt.Errorf("wrapped: %w", restic.ErrSourceUnreadable)
	fn, _ := stubBackupFn(wrapped)

	var buf bytes.Buffer

	err := backup.Run(context.Background(), targets, fn, "host1", &buf)
	if err != nil {
		t.Fatalf("want nil error for skipped target, got %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "WARN: skipping") {
		t.Errorf("want 'WARN: skipping' in output, got:\n%s", output)
	}

	if !strings.Contains(output, "0 succeeded, 1 skipped, 0 failed") {
		t.Errorf("want summary '0 succeeded, 1 skipped, 0 failed', got:\n%s", output)
	}
}

func TestRun_MixedSkipAndFail(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("ok", "/src/ok-data", "okvol"),
		makeTarget("skip", "/src/skip-data", "skipvol"),
		makeTarget("fail", "/src/fail-data", "failvol"),
	}

	wrapped := fmt.Errorf("wrapped: %w", restic.ErrSourceUnreadable)
	fn, _ := stubBackupFn(nil, wrapped, errBackup)

	var buf bytes.Buffer

	err := backup.Run(context.Background(), targets, fn, "host1", &buf)
	if err == nil {
		t.Fatal("want error because failed > 0, got nil")
	}

	if !errors.Is(err, backup.ErrTargetsFailed) {
		t.Errorf("want error wrapping ErrTargetsFailed, got %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "WARN: skipping") {
		t.Errorf("want 'WARN: skipping' in output, got:\n%s", output)
	}

	if !strings.Contains(output, "Failed") {
		t.Errorf("want 'Failed' in output, got:\n%s", output)
	}

	if !strings.Contains(output, "1 succeeded, 1 skipped, 1 failed") {
		t.Errorf("want summary '1 succeeded, 1 skipped, 1 failed', got:\n%s", output)
	}
}

func TestRun_AllSkipped(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("a", "/src/a-data", "avol"),
		makeTarget("b", "/src/b-data", "bvol"),
	}

	wrappedA := fmt.Errorf("wrapped: %w", restic.ErrSourceUnreadable)
	wrappedB := fmt.Errorf("wrapped: %w", restic.ErrSourceUnreadable)
	fn, _ := stubBackupFn(wrappedA, wrappedB)

	var buf bytes.Buffer

	err := backup.Run(context.Background(), targets, fn, "host1", &buf)
	if err != nil {
		t.Fatalf("want nil error when all targets are skipped, got %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "0 succeeded, 2 skipped, 0 failed") {
		t.Errorf("want summary '0 succeeded, 2 skipped, 0 failed', got:\n%s", output)
	}
}

func TestRun_SkipMessageFormat(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("mycontainer", "/src/some-data", "myvol"),
	}

	wrapped := fmt.Errorf("wrapped: %w", restic.ErrSourceUnreadable)
	fn, _ := stubBackupFn(wrapped)

	var buf bytes.Buffer

	err := backup.Run(context.Background(), targets, fn, "host1", &buf)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	output := buf.String()

	var skipLine string

	for line := range strings.SplitSeq(output, "\n") {
		if strings.HasPrefix(line, "WARN: skipping ") {
			skipLine = line

			break
		}
	}

	if skipLine == "" {
		t.Fatalf("want a line beginning with 'WARN: skipping ', got:\n%s", output)
	}

	// makeTarget sets Mount.Destination = "/" + mountName.
	if !strings.Contains(skipLine, "/myvol") {
		t.Errorf("want skip line to contain mount destination '/myvol', got %q", skipLine)
	}
}
