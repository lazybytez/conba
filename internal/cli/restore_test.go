package cli_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lazybytez/conba/internal/cli"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/lazybytez/conba/internal/restore"
	"github.com/lazybytez/conba/internal/runtime"
)

const testHost1 = "host1"

// --- Command-level shape tests ---

func TestNewRestoreCommand_Use(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRestoreCommand()
	if cmd.Use != "restore" {
		t.Errorf("Use = %q, want %q", cmd.Use, "restore")
	}
}

func TestNewRestoreCommand_Short(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRestoreCommand()
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestNewRestoreCommand_AllFlagsExist(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRestoreCommand()

	stringFlags := []string{"container", "volume", "snapshot", "to", "to-command"}
	for _, name := range stringFlags {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			t.Errorf("flag %q not found", name)

			continue
		}

		if flag.DefValue != "" {
			t.Errorf("flag %q default = %q, want empty", name, flag.DefValue)
		}
	}

	boolFlags := []string{"force", "all-hosts", "dry-run"}
	for _, name := range boolFlags {
		flag := cmd.Flags().Lookup(name)
		if flag == nil {
			t.Errorf("flag %q not found", name)

			continue
		}

		if flag.DefValue != "false" {
			t.Errorf("flag %q default = %q, want %q", name, flag.DefValue, "false")
		}
	}
}

func TestNewRestoreCommand_ContainerRequired(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRestoreCommand()

	flag := cmd.Flags().Lookup("container")
	if flag == nil {
		t.Fatal("container flag missing")
	}

	annotations := flag.Annotations

	required, ok := annotations["cobra_annotation_bash_completion_one_required_flag"]

	if !ok || len(required) == 0 || required[0] != "true" {
		t.Errorf("container flag should be marked required, got annotations = %v", annotations)
	}
}

func TestRestoreCommand_RegisteredOnRoot(t *testing.T) {
	t.Parallel()

	root := cli.NewRootCommand()

	for _, sub := range root.Commands() {
		if sub.Use == "restore" {
			return
		}
	}

	t.Fatal("restore command not registered on root command")
}

// --- Pre-resolution validation ---

func TestRestoreCore_VolumeAndToCommand_Rejected(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	opts := cli.RestoreCoreOptions{
		Container:        "app",
		Volume:           "data",
		Snapshot:         "",
		To:               "",
		ToCommand:        "psql -U postgres",
		Force:            false,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err == nil {
		t.Fatal("want error for --volume + --to-command, got nil")
	}

	msg := err.Error()
	if !strings.Contains(msg, "--volume") || !strings.Contains(msg, "--to-command") {
		t.Errorf("error should mention both flags, got %q", msg)
	}

	if deps.snapshotsCalled {
		t.Error("snapshotsFn should NOT be called when validation fails")
	}
}

func TestRestoreCore_ToAndToCommand_Rejected(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	opts := cli.RestoreCoreOptions{
		Container:        "app",
		Volume:           "",
		Snapshot:         "",
		To:               "/restore",
		ToCommand:        "psql -U postgres",
		Force:            false,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err == nil {
		t.Fatal("want error for --to + --to-command, got nil")
	}

	if !strings.Contains(err.Error(), "--to") || !strings.Contains(err.Error(), "--to-command") {
		t.Errorf("error should mention both flags, got %q", err.Error())
	}
}

func TestRestoreCore_ForceWithoutTo_Rejected(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	opts := cli.RestoreCoreOptions{
		Container:        "app",
		Volume:           "",
		Snapshot:         "",
		To:               "",
		ToCommand:        "",
		Force:            true,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err == nil {
		t.Fatal("want error for --force without --to, got nil")
	}

	if !strings.Contains(err.Error(), "--force") || !strings.Contains(err.Error(), "--to") {
		t.Errorf("error should mention --force and --to, got %q", err.Error())
	}
}

// --- Snapshot resolution ---

func TestRestoreCore_HostnameFilterAppliedByDefault(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{volumeSnapshot("snap1", "app", "data", testHost1)}

	opts := cli.RestoreCoreOptions{
		Container:        "app",
		Volume:           "",
		Snapshot:         "",
		To:               t.TempDir(),
		ToCommand:        "",
		Force:            false,
		AllHosts:         false,
		DryRun:           true,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !containsAll(deps.snapshotsTags, []string{"container=app", "hostname=host1"}) {
		t.Errorf("required tags = %v, want container=app + hostname=host1", deps.snapshotsTags)
	}
}

func TestRestoreCore_AllHosts_DropsHostnameFilter(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{volumeSnapshot("snap1", "app", "data", "host2")}

	opts := cli.RestoreCoreOptions{
		Container:        "app",
		Volume:           "",
		Snapshot:         "",
		To:               t.TempDir(),
		ToCommand:        "",
		Force:            false,
		AllHosts:         true,
		DryRun:           true,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, tag := range deps.snapshotsTags {
		if strings.HasPrefix(tag, "hostname=") {
			t.Errorf("hostname tag should not be present, got %q", tag)
		}
	}
}

func TestRestoreCore_VolumeFlag_AddedToFilter(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{volumeSnapshot("snap1", "app", "data", testHost1)}

	opts := cli.RestoreCoreOptions{
		Container:        "app",
		Volume:           "data",
		Snapshot:         "",
		To:               t.TempDir(),
		ToCommand:        "",
		Force:            false,
		AllHosts:         false,
		DryRun:           true,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantTags := []string{"container=app", "volume=data", "hostname=host1"}
	if !containsAll(deps.snapshotsTags, wantTags) {
		t.Errorf("required tags = %v, want %v", deps.snapshotsTags, wantTags)
	}
}

func TestRestoreCore_NoMatchingSnapshot_Error(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = nil

	opts := cli.RestoreCoreOptions{
		Container:        "app",
		Volume:           "",
		Snapshot:         "",
		To:               t.TempDir(),
		ToCommand:        "",
		Force:            false,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err == nil {
		t.Fatal("want error when no snapshot matches, got nil")
	}

	if !strings.Contains(err.Error(), "no snapshot") {
		t.Errorf("error should mention no snapshot, got %q", err.Error())
	}

	if !strings.Contains(err.Error(), "container=app") {
		t.Errorf("error should mention filters tried, got %q", err.Error())
	}
}

func TestRestoreCore_ExplicitSnapshotID_TagMismatch(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{
		volumeSnapshot("snapX", "other", "data", testHost1),
	}

	opts := cli.RestoreCoreOptions{
		Container:        "app",
		Volume:           "",
		Snapshot:         "snapX",
		To:               t.TempDir(),
		ToCommand:        "",
		Force:            false,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err == nil {
		t.Fatal("want error for tag mismatch, got nil")
	}

	if !errors.Is(err, restic.ErrSnapshotTagMismatch) {
		t.Errorf("want ErrSnapshotTagMismatch, got %v", err)
	}
}

// --- Volume mode ---

func TestRestoreCore_VolumeMode_RequiresTo(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{volumeSnapshot("snap1", "app", "data", testHost1)}

	opts := cli.RestoreCoreOptions{
		Container:        "app",
		Volume:           "",
		Snapshot:         "",
		To:               "",
		ToCommand:        "",
		Force:            false,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err == nil {
		t.Fatal("want error for missing --to in volume mode, got nil")
	}

	if !strings.Contains(err.Error(), "--to") {
		t.Errorf("error should mention --to, got %q", err.Error())
	}
}

func TestRestoreCore_VolumeMode_HappyPath_DryRun(t *testing.T) {
	t.Parallel()

	target := t.TempDir()
	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{volumeSnapshot("snap1", "app", "data", testHost1)}

	opts := cli.RestoreCoreOptions{
		Container:        "app",
		Volume:           "data",
		Snapshot:         "",
		To:               target,
		ToCommand:        "",
		Force:            false,
		AllHosts:         false,
		DryRun:           true,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !deps.restoreCalled {
		t.Fatal("restoreFn should be called for volume mode")
	}

	if deps.restoreSnapshotID != "snap1" {
		t.Errorf("snapshotID = %q, want snap1", deps.restoreSnapshotID)
	}

	if deps.restoreTargetPath != target {
		t.Errorf("targetPath = %q, want %q", deps.restoreTargetPath, target)
	}

	if !deps.restoreDryRun {
		t.Error("restoreFn should receive dryRun=true")
	}

	if deps.dumpCalled {
		t.Error("dumpFn must NOT be called for volume mode")
	}
}

func TestRestoreCore_VolumeMode_RejectsToCommand(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{volumeSnapshot("snap1", "app", "data", testHost1)}

	opts := cli.RestoreCoreOptions{
		Container:        "app",
		Volume:           "data",
		Snapshot:         "",
		To:               "",
		ToCommand:        "psql -U postgres",
		Force:            false,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err == nil {
		t.Fatal("want error for --to-command on volume snapshot, got nil")
	}

	// pre-resolution validation rejects volume+to-command early; either
	// pre-validation or post-resolution wording is acceptable.
	if !strings.Contains(err.Error(), "--to-command") {
		t.Errorf("error should mention --to-command, got %q", err.Error())
	}
}

func TestRestoreCore_VolumeMode_MultipleVolumes_RequiresVolumeFlag(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{
		volumeSnapshot("snap1", "app", "data", testHost1),
		volumeSnapshot("snap2", "app", "config", testHost1),
	}

	opts := cli.RestoreCoreOptions{
		Container:        "app",
		Volume:           "",
		Snapshot:         "",
		To:               t.TempDir(),
		ToCommand:        "",
		Force:            false,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err == nil {
		t.Fatal("want error when multiple volumes match without --volume, got nil")
	}

	if !strings.Contains(err.Error(), "--volume") {
		t.Errorf("error should mention --volume, got %q", err.Error())
	}

	if !strings.Contains(err.Error(), "data") || !strings.Contains(err.Error(), "config") {
		t.Errorf("error should list distinct volume names, got %q", err.Error())
	}
}

// --- Stream mode ---

func TestRestoreCore_StreamMode_RejectsTo(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{
		streamSnapshot(),
	}

	opts := cli.RestoreCoreOptions{
		Container:        "db",
		Volume:           "",
		Snapshot:         "",
		To:               "/restore/here",
		ToCommand:        "",
		Force:            false,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err == nil {
		t.Fatal("want error for --to on stream snapshot, got nil")
	}

	if !strings.Contains(err.Error(), "stream") || !strings.Contains(err.Error(), "--to-command") {
		t.Errorf("error should mention stream + --to-command, got %q", err.Error())
	}
}

func TestRestoreCore_StreamMode_RejectsVolume(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{
		streamSnapshot(),
	}

	opts := cli.RestoreCoreOptions{
		Container:        "db",
		Volume:           "data",
		Snapshot:         "",
		To:               "",
		ToCommand:        "psql -U postgres",
		Force:            false,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err == nil {
		t.Fatal("want error for --volume on stream snapshot, got nil")
	}

	if !strings.Contains(err.Error(), "--volume") {
		t.Errorf("error should mention --volume, got %q", err.Error())
	}
}

func TestRestoreCore_StreamMode_FlagWinsOverLabel(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{
		streamSnapshot(),
	}
	deps.lookupContainer = func(_ context.Context, _ string) (runtime.ContainerInfo, error) {
		return runtime.ContainerInfo{
			ID:   "c1",
			Name: "db",
			Labels: map[string]string{
				filter.LabelPreBackupCommand:        "pg_dumpall",
				filter.LabelPreBackupRestoreCommand: "psql -U postgres-LABEL",
			},
			Mounts: nil,
		}, nil
	}

	var buf bytes.Buffer

	opts := cli.RestoreCoreOptions{
		Container:        "db",
		Volume:           "",
		Snapshot:         "",
		To:               "",
		ToCommand:        "psql -U postgres-FLAG",
		Force:            false,
		AllHosts:         false,
		DryRun:           true,
		PreBackupEnabled: true,
		Out:              &buf,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "psql -U postgres-FLAG") {
		t.Errorf("dry-run output should mention flag value, got %q", buf.String())
	}

	if strings.Contains(buf.String(), "psql -U postgres-LABEL") {
		t.Errorf("dry-run output should NOT mention label value, got %q", buf.String())
	}

	if deps.execCalled {
		t.Error("dry-run must NOT invoke deps.Exec")
	}
}

func TestRestoreCore_StreamMode_LabelUsedWhenFeatureEnabled(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{
		streamSnapshot(),
	}
	deps.lookupContainer = func(_ context.Context, name string) (runtime.ContainerInfo, error) {
		return runtime.ContainerInfo{
			ID:   "c1",
			Name: name,
			Labels: map[string]string{
				filter.LabelPreBackupCommand:        "pg_dumpall",
				filter.LabelPreBackupRestoreCommand: "psql -U postgres",
			},
			Mounts: nil,
		}, nil
	}

	var buf bytes.Buffer

	opts := cli.RestoreCoreOptions{
		Container:        "db",
		Volume:           "",
		Snapshot:         "",
		To:               "",
		ToCommand:        "",
		Force:            false,
		AllHosts:         false,
		DryRun:           true,
		PreBackupEnabled: true,
		Out:              &buf,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(buf.String(), "psql -U postgres") {
		t.Errorf("dry-run output should mention label value, got %q", buf.String())
	}

	if deps.execCalled {
		t.Error("dry-run must NOT invoke deps.Exec")
	}
}

func TestRestoreCore_StreamMode_LabelIgnoredWhenFeatureDisabled(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{
		streamSnapshot(),
	}
	deps.lookupContainer = func(_ context.Context, name string) (runtime.ContainerInfo, error) {
		return runtime.ContainerInfo{
			ID:   "c1",
			Name: name,
			Labels: map[string]string{
				filter.LabelPreBackupCommand:        "pg_dumpall",
				filter.LabelPreBackupRestoreCommand: "psql -U postgres",
			},
			Mounts: nil,
		}, nil
	}

	opts := cli.RestoreCoreOptions{
		Container:        "db",
		Volume:           "",
		Snapshot:         "",
		To:               "",
		ToCommand:        "",
		Force:            false,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err == nil {
		t.Fatal("want error when label ignored and no flag set, got nil")
	}

	if !strings.Contains(err.Error(), "no restore command") {
		t.Errorf("error should mention no restore command, got %q", err.Error())
	}
}

func TestRestoreCore_StreamMode_NoCommandAvailable(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{
		streamSnapshot(),
	}
	deps.lookupContainer = func(_ context.Context, name string) (runtime.ContainerInfo, error) {
		return runtime.ContainerInfo{
			ID:     "c1",
			Name:   name,
			Labels: map[string]string{},
			Mounts: nil,
		}, nil
	}

	opts := cli.RestoreCoreOptions{
		Container:        "db",
		Volume:           "",
		Snapshot:         "",
		To:               "",
		ToCommand:        "",
		Force:            false,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: true,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err == nil {
		t.Fatal("want error when no restore command available, got nil")
	}

	if !strings.Contains(err.Error(), "no restore command") {
		t.Errorf("error should mention no restore command, got %q", err.Error())
	}
}

func TestRestoreCore_StreamMode_HappyPathLive(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{
		streamSnapshot(),
	}

	opts := cli.RestoreCoreOptions{
		Container:        "db",
		Volume:           "",
		Snapshot:         "",
		To:               "",
		ToCommand:        "psql -U postgres",
		Force:            false,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !deps.dumpCalled {
		t.Fatal("dumpFn should be called for stream mode")
	}

	if deps.dumpSnapshotID != "snap1" {
		t.Errorf("dump snapshotID = %q, want snap1", deps.dumpSnapshotID)
	}

	if deps.dumpFilename != "/dump.sql" {
		t.Errorf("dump filename = %q, want /dump.sql", deps.dumpFilename)
	}

	if deps.execCommand != "psql -U postgres" {
		t.Errorf("exec command = %q, want %q", deps.execCommand, "psql -U postgres")
	}

	wantArgv := []string{"docker", "exec", "-i", "db", "sh", "-c", "psql -U postgres"}
	if !reflect.DeepEqual(deps.execArgv, wantArgv) {
		t.Errorf("exec argv = %v, want %v", deps.execArgv, wantArgv)
	}
}

// --- Error mapping ---

func TestRestoreCore_DestinationNotEmpty_FriendlyMessage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// Make destination non-empty so RunVolume returns ErrDestinationNotEmpty.
	mustWriteSentinel(t, dir)

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{volumeSnapshot("snap1", "app", "data", testHost1)}

	opts := cli.RestoreCoreOptions{
		Container:        "app",
		Volume:           "data",
		Snapshot:         "",
		To:               dir,
		ToCommand:        "",
		Force:            false,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err == nil {
		t.Fatal("want error from non-empty dir, got nil")
	}

	if !strings.Contains(err.Error(), dir) {
		t.Errorf("error should mention path, got %q", err.Error())
	}

	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should mention --force, got %q", err.Error())
	}
}

func TestRestoreCore_ContainerNotRunning_FriendlyMessage(t *testing.T) {
	t.Parallel()

	deps := newRestoreStubs()
	deps.hostname = testHost1
	deps.snapshotsResult = []restic.Snapshot{
		streamSnapshot(),
	}
	deps.containerRunning = false

	opts := cli.RestoreCoreOptions{
		Container:        "db",
		Volume:           "",
		Snapshot:         "",
		To:               "",
		ToCommand:        "psql",
		Force:            false,
		AllHosts:         false,
		DryRun:           false,
		PreBackupEnabled: false,
		Out:              io.Discard,
	}

	err := cli.RunRestoreCore(context.Background(), opts, deps)
	if err == nil {
		t.Fatal("want error when container not running, got nil")
	}

	if !errors.Is(err, restore.ErrContainerNotRunning) {
		t.Errorf("want ErrContainerNotRunning, got %v", err)
	}

	if !strings.Contains(err.Error(), "db") {
		t.Errorf("error should mention container name, got %q", err.Error())
	}
}

// --- Helpers ---

// volumeSnapshot builds a snapshot tagged as a volume snapshot for tests.
func volumeSnapshot(id, container, volume, hostname string) restic.Snapshot {
	return restic.Snapshot{
		ID:       id,
		Time:     time.Now(),
		Paths:    []string{"/var/lib/docker/volumes/" + volume + "/_data"},
		Tags:     []string{"container=" + container, "volume=" + volume, "hostname=" + hostname},
		Hostname: hostname,
	}
}

// streamSnapshot builds a snapshot tagged as a stream snapshot for tests.
// Container "db", hostname testHost1, path "/dump.sql", and ID "snap1" are
// hard-coded; all call sites use the same values.
func streamSnapshot() restic.Snapshot {
	return restic.Snapshot{
		ID:       "snap1",
		Time:     time.Now(),
		Paths:    []string{"/dump.sql"},
		Tags:     []string{"container=db", "hostname=host1", "kind=stream"},
		Hostname: testHost1,
	}
}

func containsAll(haystack []string, needles []string) bool {
	for _, n := range needles {
		if !slices.Contains(haystack, n) {
			return false
		}
	}

	return true
}

func mustWriteSentinel(t *testing.T, dir string) {
	t.Helper()

	f, err := openSentinel(dir)
	if err != nil {
		t.Fatalf("create sentinel: %v", err)
	}

	closeErr := f.Close()
	if closeErr != nil {
		t.Fatalf("close sentinel: %v", closeErr)
	}
}

func newRestoreStubs() *restoreStubs {
	stubs := &restoreStubs{
		hostname:            "",
		snapshotsCalled:     false,
		snapshotsTags:       nil,
		snapshotsResult:     nil,
		snapshotsErr:        nil,
		lookupContainer:     nil,
		restoreCalled:       false,
		restoreSnapshotID:   "",
		restoreTargetPath:   "",
		restoreDryRun:       false,
		restoreErr:          nil,
		dumpCalled:          false,
		dumpSnapshotID:      "",
		dumpFilename:        "",
		dumpPayload:         nil,
		dumpErr:             nil,
		containerRunning:    true,
		containerRunningErr: nil,
		execCalled:          false,
		execName:            "",
		execArgv:            nil,
		execCommand:         "",
		execErr:             nil,
	}
	stubs.lookupContainer = func(_ context.Context, name string) (runtime.ContainerInfo, error) {
		return runtime.ContainerInfo{
			ID:     "c-" + name,
			Name:   name,
			Labels: map[string]string{},
			Mounts: nil,
		}, nil
	}

	return stubs
}

// restoreStubs implements cli.RestoreCoreDeps via methods exposed in
// export_test.go. It captures inputs to all dependency calls so tests can
// assert orchestration order and argument propagation.
type restoreStubs struct {
	hostname string

	snapshotsCalled bool
	snapshotsTags   []string
	snapshotsResult []restic.Snapshot
	snapshotsErr    error

	lookupContainer func(ctx context.Context, name string) (runtime.ContainerInfo, error)

	restoreCalled     bool
	restoreSnapshotID string
	restoreTargetPath string
	restoreDryRun     bool
	restoreErr        error

	dumpCalled     bool
	dumpSnapshotID string
	dumpFilename   string
	dumpPayload    []byte
	dumpErr        error

	containerRunning    bool
	containerRunningErr error

	execCalled  bool
	execName    string
	execArgv    []string
	execCommand string
	execErr     error
}

// Hostname implements cli.RestoreCoreDeps.
func (s *restoreStubs) Hostname() (string, error) {
	if s.hostname == "" {
		return "host-default", nil
	}

	return s.hostname, nil
}

// Snapshots implements cli.RestoreCoreDeps.
func (s *restoreStubs) Snapshots(_ context.Context, tags []string) ([]restic.Snapshot, error) {
	captured := append([]string(nil), tags...)

	s.snapshotsCalled = true
	s.snapshotsTags = captured

	return s.snapshotsResult, s.snapshotsErr
}

// LookupContainer implements cli.RestoreCoreDeps.
func (s *restoreStubs) LookupContainer(
	ctx context.Context, name string,
) (runtime.ContainerInfo, error) {
	return s.lookupContainer(ctx, name)
}

// Restore implements cli.RestoreCoreDeps.
func (s *restoreStubs) Restore(
	_ context.Context, snapshotID, targetPath string, dryRun bool,
) error {
	s.restoreCalled = true
	s.restoreSnapshotID = snapshotID
	s.restoreTargetPath = targetPath
	s.restoreDryRun = dryRun

	return s.restoreErr
}

// Dump implements cli.RestoreCoreDeps.
func (s *restoreStubs) Dump(
	_ context.Context, snapshotID, filename string, stdout io.Writer,
) error {
	s.dumpCalled = true
	s.dumpSnapshotID = snapshotID
	s.dumpFilename = filename

	payload := s.dumpPayload
	if payload == nil {
		payload = []byte("STUB-DATA")
	}

	_, _ = stdout.Write(payload)

	return s.dumpErr
}

// ContainerRunning implements cli.RestoreCoreDeps via the DockerRuntime side.
func (s *restoreStubs) ContainerRunning(_ context.Context, _ string) (bool, error) {
	return s.containerRunning, s.containerRunningErr
}

// Exec implements cli.RestoreCoreDeps via the DockerRuntime side.
func (s *restoreStubs) Exec(_ context.Context, name string, argv []string, stdin io.Reader) error {
	captured := append([]string(nil), argv...)

	s.execCalled = true
	s.execName = name
	s.execArgv = captured

	if len(argv) >= 7 {
		s.execCommand = argv[6]
	}

	if stdin != nil {
		_, _ = io.ReadAll(stdin)
	}

	return s.execErr
}

// Compile-time assertion that the stubs satisfy the deps contract.
var _ cli.RestoreCoreDeps = (*restoreStubs)(nil)

// out implements io.Writer aliasing for tests that don't care about output.
type discardOut struct{}

func (discardOut) Write(p []byte) (int, error) { return len(p), nil }

// keep imports we haven't directly referenced in the tail.
var (
	_ = bytes.Buffer{}
	_ = discardOut{}
)
