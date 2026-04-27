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

func makeLabeledTarget(
	source, mountName string,
	labels map[string]string,
) discovery.Target {
	target := makeTarget("mysql", source, mountName)
	target.Container.Labels = labels

	return target
}

func nilStreamFn() backup.StreamFunc {
	return nil
}

func recordingStreamFn(err error) (backup.StreamFunc, *[]string) {
	var calls []string

	return func(_ context.Context, filename string, _ []string, _ []string) error {
		calls = append(calls, filename)

		return err
	}, &calls
}

func TestRun_AllSucceed(t *testing.T) {
	t.Parallel()

	targets := []discovery.Target{
		makeTarget("app", "/src/app-data", "data"),
		makeTarget("db", "/src/db-data", "pgdata"),
	}

	fn, _ := stubBackupFn(nil, nil)

	var buf bytes.Buffer

	err := backup.Run(context.Background(), targets, fn, nilStreamFn(), false, "host1", &buf)
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

	err := backup.Run(context.Background(), targets, fn, nilStreamFn(), false, "host1", &buf)
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

	err := backup.Run(context.Background(), targets, fn, nilStreamFn(), false, "host1", &buf)
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

	err := backup.Run(context.Background(), nil, fn, nilStreamFn(), false, "host1", &buf)
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

	err := backup.Run(context.Background(), targets, fn, nilStreamFn(), false, "host1", &buf)
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

	err := backup.Run(
		context.Background(), targets, backupFn, nilStreamFn(),
		false, "testhost", &buf,
	)
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

	err := backup.Run(context.Background(), targets, fn, nilStreamFn(), false, "host1", &buf)
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

	err := backup.Run(context.Background(), targets, fn, nilStreamFn(), false, "host1", &buf)
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

	err := backup.Run(context.Background(), targets, fn, nilStreamFn(), false, "host1", &buf)
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

	err := backup.Run(context.Background(), targets, fn, nilStreamFn(), false, "host1", &buf)
	if err != nil {
		t.Fatalf("want nil error when all targets are skipped, got %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "0 succeeded, 2 skipped, 0 failed") {
		t.Errorf("want summary '0 succeeded, 2 skipped, 0 failed', got:\n%s", output)
	}
}

func TestRun_PreBackupDisabled_IgnoresLabels(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command": "mysqldump --all-databases",
	}
	targets := []discovery.Target{
		makeLabeledTarget("/src/mysql-data", "data", labels),
	}

	backupFn, paths := stubBackupFn(nil)
	streamFn, streamCalls := recordingStreamFn(nil)

	var buf bytes.Buffer

	err := backup.Run(
		context.Background(),
		targets,
		backupFn,
		streamFn,
		false,
		"host1",
		&buf,
	)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if len(*streamCalls) != 0 {
		t.Errorf("want 0 stream calls when feature disabled, got %d", len(*streamCalls))
	}

	if len(*paths) != 1 {
		t.Errorf("want 1 volume backup call when feature disabled, got %d", len(*paths))
	}

	output := buf.String()
	if !strings.Contains(output, "Backed up mysql/data") {
		t.Errorf("want 'Backed up mysql/data' line, got:\n%s", output)
	}
}

func TestRun_AlongsideMode_StreamAndVolumeBothRun(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command": "mysqldump --all-databases",
		"conba.pre-backup.mode":    "alongside",
	}
	targets := []discovery.Target{
		makeLabeledTarget("/src/mysql-data", "data", labels),
	}

	backupFn, paths := stubBackupFn(nil)
	streamFn, streamCalls := recordingStreamFn(nil)

	var buf bytes.Buffer

	err := backup.Run(
		context.Background(),
		targets,
		backupFn,
		streamFn,
		true,
		"host1",
		&buf,
	)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if len(*streamCalls) != 1 {
		t.Errorf("want 1 stream call in alongside mode, got %d", len(*streamCalls))
	}

	if len(*paths) != 1 {
		t.Errorf("want 1 volume backup call in alongside mode, got %d", len(*paths))
	}

	output := buf.String()
	if !strings.Contains(output, "Backed up mysql stream") {
		t.Errorf("want 'Backed up mysql stream' in output, got:\n%s", output)
	}

	if !strings.Contains(output, "Backed up mysql/data") {
		t.Errorf("want 'Backed up mysql/data' in output, got:\n%s", output)
	}
}

func TestRun_ReplaceMode_MultipleMounts_StreamRunsOnce(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command": "mysqldump",
	}
	target1 := makeLabeledTarget("/src/mysql-data", "data", labels)
	target2 := makeLabeledTarget("/src/mysql-conf", "conf", labels)
	targets := []discovery.Target{target1, target2}

	backupFn, paths := stubBackupFn()
	streamFn, streamCalls := recordingStreamFn(nil)

	var buf bytes.Buffer

	err := backup.Run(
		context.Background(),
		targets,
		backupFn,
		streamFn,
		true,
		"host1",
		&buf,
	)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if len(*streamCalls) != 1 {
		t.Errorf("want exactly 1 stream call for 2-mount group, got %d", len(*streamCalls))
	}

	if len(*paths) != 0 {
		t.Errorf("want 0 volume backups in replace mode, got %d", len(*paths))
	}

	output := buf.String()
	if strings.Count(output, "replaced by pre-backup stream") != 2 {
		t.Errorf("want 2 'replaced' messages (one per mount), got:\n%s", output)
	}
}

func TestRun_StreamFails_Replace_TargetGroupFailed(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command": "false",
	}
	targets := []discovery.Target{
		makeLabeledTarget("/src/mysql-data", "data", labels),
		// Independent unlabeled container — must still complete after failure.
		makeTarget("app", "/src/app-data", "appvol"),
	}

	backupFn, paths := stubBackupFn(nil)
	streamFn, _ := recordingStreamFn(errStream)

	var buf bytes.Buffer

	err := backup.Run(
		context.Background(),
		targets,
		backupFn,
		streamFn,
		true,
		"host1",
		&buf,
	)
	if err == nil {
		t.Fatal("want error because stream failed, got nil")
	}

	if !errors.Is(err, backup.ErrTargetsFailed) {
		t.Errorf("want error wrapping ErrTargetsFailed, got %v", err)
	}

	if len(*paths) != 1 {
		t.Errorf(
			"want 1 volume backup call (only the unlabeled container), got %d",
			len(*paths),
		)
	}

	output := buf.String()
	if !strings.Contains(output, "Failed mysql stream") {
		t.Errorf("want 'Failed mysql stream' in output, got:\n%s", output)
	}

	if !strings.Contains(output, "Backed up app/appvol") {
		t.Errorf("want 'Backed up app/appvol' in output (cycle continues), got:\n%s", output)
	}
}

func TestRun_StreamFails_Alongside_VolumeStillRuns(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command": "false",
		"conba.pre-backup.mode":    "alongside",
	}
	targets := []discovery.Target{
		makeLabeledTarget("/src/mysql-data", "data", labels),
	}

	backupFn, paths := stubBackupFn(nil)
	streamFn, _ := recordingStreamFn(errStream)

	var buf bytes.Buffer

	err := backup.Run(
		context.Background(),
		targets,
		backupFn,
		streamFn,
		true,
		"host1",
		&buf,
	)
	if err == nil {
		t.Fatal("want error because stream failed, got nil")
	}

	if len(*paths) != 1 {
		t.Errorf("want 1 volume backup call (alongside continues), got %d", len(*paths))
	}

	output := buf.String()
	if !strings.Contains(output, "Failed mysql stream") {
		t.Errorf("want 'Failed mysql stream' in output, got:\n%s", output)
	}

	if !strings.Contains(output, "Backed up mysql/data") {
		t.Errorf("want 'Backed up mysql/data' in output, got:\n%s", output)
	}
}

func TestRun_InvalidMode_GroupFailed_CycleContinues(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command": "mysqldump",
		"conba.pre-backup.mode":    "garbage",
	}
	targets := []discovery.Target{
		makeLabeledTarget("/src/mysql-data", "data", labels),
		makeTarget("app", "/src/app-data", "appvol"),
	}

	backupFn, paths := stubBackupFn(nil)
	streamFn, streamCalls := recordingStreamFn(nil)

	var buf bytes.Buffer

	err := backup.Run(
		context.Background(),
		targets,
		backupFn,
		streamFn,
		true,
		"host1",
		&buf,
	)
	if err == nil {
		t.Fatal("want error because invalid mode, got nil")
	}

	if !errors.Is(err, backup.ErrTargetsFailed) {
		t.Errorf("want error wrapping ErrTargetsFailed, got %v", err)
	}

	if len(*streamCalls) != 0 {
		t.Errorf("want 0 stream calls on invalid mode, got %d", len(*streamCalls))
	}

	if len(*paths) != 1 {
		t.Errorf("want 1 volume backup (unlabeled cycle continues), got %d", len(*paths))
	}

	output := buf.String()
	if !strings.Contains(output, "invalid pre-backup labels") {
		t.Errorf("want 'invalid pre-backup labels' in output, got:\n%s", output)
	}

	if !strings.Contains(output, "Backed up app/appvol") {
		t.Errorf("want 'Backed up app/appvol' (cycle continues), got:\n%s", output)
	}
}

func TestRun_ReplaceMode_SkipsVolumeBackup(t *testing.T) {
	t.Parallel()

	labels := map[string]string{
		"conba.pre-backup.command": "mysqldump --all-databases",
	}
	targets := []discovery.Target{
		makeLabeledTarget("/src/mysql-data", "data", labels),
	}

	backupFn, paths := stubBackupFn()
	streamFn, streamCalls := recordingStreamFn(nil)

	var buf bytes.Buffer

	err := backup.Run(
		context.Background(),
		targets,
		backupFn,
		streamFn,
		true,
		"host1",
		&buf,
	)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}

	if len(*paths) != 0 {
		t.Errorf("want 0 volume backup calls in replace mode, got %d", len(*paths))
	}

	if len(*streamCalls) != 1 {
		t.Errorf("want 1 stream call, got %d", len(*streamCalls))
	}

	output := buf.String()

	if !strings.Contains(output, "replaced by pre-backup stream") {
		t.Errorf("want 'replaced by pre-backup stream' in output, got:\n%s", output)
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

	err := backup.Run(context.Background(), targets, fn, nilStreamFn(), false, "host1", &buf)
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
