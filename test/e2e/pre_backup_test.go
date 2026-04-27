//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	tagKindStream = "kind=stream"

	// preBackupNetwork is the docker network the e2e fixture uses; ad-hoc
	// containers must join it so conba's discovery sees them through the
	// shared docker daemon socket.
	preBackupNetwork = "conba-e2e"

	// preBackupImage is the small image used for ad-hoc test containers.
	// It must already be pulled by the compose fixture, so we reuse the
	// same alpine tag the fixture's app/ignored services use.
	preBackupImage = "docker.io/library/alpine:3.20.10"

	// preBackupDumpPayload is the deterministic stdout produced by the
	// happy-path dump command. Snapshot content is verified against this.
	preBackupDumpPayload = "conba-e2e-dump-payload"
)

// preBackupContainerOpts configures an ad-hoc labeled container. Used by
// every scenario in this file, so the helper below is justified by reuse.
type preBackupContainerOpts struct {
	Name       string
	Labels     map[string]string
	VolumeName string
	Command    []string
}

// startPreBackupContainer launches a labeled container on the e2e network
// and registers cleanup to remove it (and any named volume) on test exit.
// The container runs `tail -f /dev/null` by default so the labels are
// observable to conba's discovery for the duration of the test. Reused
// across all six scenarios in this file.
func startPreBackupContainer(t *testing.T, opts preBackupContainerOpts) {
	t.Helper()

	args := []string{
		"run", "-d",
		"--name", opts.Name,
		"--network", preBackupNetwork,
	}

	for key, value := range opts.Labels {
		args = append(args, "--label", key+"="+value)
	}

	if opts.VolumeName != "" {
		args = append(args, "-v", opts.VolumeName+":/data")
	}

	args = append(args, preBackupImage)

	if len(opts.Command) == 0 {
		args = append(args, "sh", "-c", "tail -f /dev/null")
	} else {
		args = append(args, opts.Command...)
	}

	runDocker(t, args...)

	t.Cleanup(func() {
		runDockerIgnoreErr("rm", "-f", opts.Name)

		if opts.VolumeName != "" {
			runDockerIgnoreErr("volume", "rm", "-f", opts.VolumeName)
		}
	})
}

// runDocker shells out to `docker` with the supplied args. Failures fail
// the test. Used for fixture setup that the existing helpers do not cover.
func runDocker(t *testing.T, args ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker %v: %v: %s", args, err, strings.TrimSpace(string(out)))
	}
}

// runDockerIgnoreErr shells out to `docker` and discards any error. Used
// only from t.Cleanup where a failure to clean up a stray container or
// volume must not mask the test's real verdict.
func runDockerIgnoreErr(args ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()

	_ = exec.CommandContext(ctx, "docker", args...).Run()
}

// writePreBackupConfig renders a conba.yaml configured for pre-backup
// scenarios. It enables the feature flag (when enabled is true) and
// passes a restic environment block — `restic --stdin-from-command`
// spawns docker as a child and refuses to start without HOME or
// XDG_CACHE_HOME, so we set both PATH (to find docker) and
// RESTIC_CACHE_DIR (per-test scratch cache).
func writePreBackupConfig(
	t *testing.T,
	dir, repoPath string,
	includePatterns []string,
	enabled bool,
) {
	t.Helper()

	cacheDir := filepath.Join(dir, "restic-cache")

	err := os.MkdirAll(cacheDir, 0o700)
	if err != nil {
		t.Fatalf("create restic cache dir: %v", err)
	}

	writeConfig(t, dir, configOpts{
		ResticRepoPath:      repoPath,
		ResticPassword:      "",
		IncludeNames:        nil,
		IncludeNamePatterns: includePatterns,
		ExcludeNames:        nil,
		ResticEnvironment: map[string]string{
			"PATH":             os.Getenv("PATH"),
			"RESTIC_CACHE_DIR": cacheDir,
		},
		PreBackupCommandsEnabled: enabled,
	})
}

// streamSnapshotsOf returns the subset of snaps that carry the kind=stream
// tag. Used by every scenario in this file to filter stream snapshots
// from the surrounding volume snapshots in the same repo.
func streamSnapshotsOf(snaps []ResticSnapshot) []ResticSnapshot {
	var matches []ResticSnapshot

	for _, snap := range snaps {
		if slices.Contains(snap.Tags, tagKindStream) {
			matches = append(matches, snap)
		}
	}

	return matches
}

// resticDump invokes `restic dump <snapshot> <filename>` and returns the
// captured stdout bytes. Used to verify that stream snapshots store the
// dump command's output verbatim under the configured filename.
func resticDump(t *testing.T, repoPath, snapshotID, filename string) []byte {
	t.Helper()

	stdout, stderr, err := runRestic(t, repoPath, "dump", snapshotID, filename)
	if err != nil {
		t.Fatalf(
			"restic dump %s %s: %v: %s",
			snapshotID, filename, err, strings.TrimSpace(stderr),
		)
	}

	return []byte(stdout)
}

// uniqueName returns a name derived from the test name plus the current
// time, suitable for use as a docker container or volume name. Avoids
// collisions when tests run sequentially against a long-lived fixture.
func uniqueName(t *testing.T, prefix string) string {
	t.Helper()

	return fmt.Sprintf("%s-%s-%d", prefix, sanitize(t.Name()), time.Now().UnixNano())
}

// sanitize replaces characters that are not valid in docker resource names.
func sanitize(input string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			return r
		default:
			return '-'
		}
	}, strings.ToLower(input))
}

// preBackupSetup is the boilerplate every scenario shares: reset the
// fixture, allocate a temp dir, render the conba.yaml restricted to the
// supplied include patterns, and run `conba init`.
func preBackupSetup(
	t *testing.T,
	includePatterns []string,
	enabled bool,
) (runConfig, string) {
	t.Helper()

	resetFixture(t)

	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")

	writePreBackupConfig(t, dir, repoPath, includePatterns, enabled)

	cfg := runConfig{Dir: dir, Stdin: nil, Env: nil}

	initResult := runConba(t, cfg, "init")
	requireSuccess(t, initResult, "conba init")

	return cfg, repoPath
}

// TestPreBackup_ReplaceMode_ProducesOnlyStreamSnapshot was the first
// scenario implemented and confirmed against `make e2e` before adding the
// remaining cases. It verifies the spec's headline behaviour: in replace
// mode (the default), a labeled container yields exactly one kind=stream
// snapshot per cycle and no volume snapshot for its eligible mount, and
// the stored file matches the dump command's stdout.
func TestPreBackup_ReplaceMode_ProducesOnlyStreamSnapshot(t *testing.T) {
	containerName := uniqueName(t, "conba-e2e-pb")
	volumeName := containerName + "-data"

	startPreBackupContainer(t, preBackupContainerOpts{
		Name:       containerName,
		VolumeName: volumeName,
		Command:    nil,
		Labels: map[string]string{
			"conba.pre-backup.command":  "printf '" + preBackupDumpPayload + "'",
			"conba.pre-backup.filename": "dump.txt",
		},
	})

	cfg, repoPath := preBackupSetup(t, []string{"^" + containerName + "$"}, true)

	backupResult := runConba(t, cfg, "backup")
	requireSuccess(t, backupResult, "conba backup")

	snaps := resticSnapshots(t, repoPath)
	streamSnap := requireSingleStreamSnapshot(t, snaps, containerName)

	if hasTagWithPrefix(streamSnap.Tags, tagPrefixVolume) {
		t.Fatalf(
			"replace mode: stream snapshot must not carry a volume= tag; tags=%v",
			streamSnap.Tags,
		)
	}

	dumpBytes := resticDump(t, repoPath, streamSnap.ID, "dump.txt")
	if !bytes.Equal(dumpBytes, []byte(preBackupDumpPayload)) {
		t.Fatalf(
			"replace mode: stored stream content = %q, want %q",
			string(dumpBytes), preBackupDumpPayload,
		)
	}

	requireStdoutContains(t, backupResult, "Backed up "+containerName+" stream")
	requireStdoutContains(
		t, backupResult,
		"Skipped "+containerName+"/"+volumeName+": replaced by pre-backup stream",
	)
}

// TestPreBackup_AlongsideMode_ProducesStreamAndVolumeSnapshots verifies
// that mode=alongside yields one stream snapshot AND a volume snapshot
// for each eligible mount on the labeled container.
func TestPreBackup_AlongsideMode_ProducesStreamAndVolumeSnapshots(t *testing.T) {
	containerName := uniqueName(t, "conba-e2e-pb-alongside")
	volumeName := containerName + "-data"

	startPreBackupContainer(t, preBackupContainerOpts{
		Name:       containerName,
		VolumeName: volumeName,
		Command:    nil,
		Labels: map[string]string{
			"conba.pre-backup.command": "printf '" + preBackupDumpPayload + "'",
			"conba.pre-backup.mode":    "alongside",
		},
	})

	cfg, repoPath := preBackupSetup(t, []string{"^" + containerName + "$"}, true)

	backupResult := runConba(t, cfg, "backup")
	requireSuccess(t, backupResult, "conba backup")

	snaps := resticSnapshots(t, repoPath)

	containerSnaps := snapshotsForContainer(snaps, containerName)
	if len(containerSnaps) != 2 {
		t.Fatalf(
			"alongside mode: want 2 snapshots for %s (1 stream + 1 volume), got %d (tags=%v)",
			containerName, len(containerSnaps), allTags(containerSnaps),
		)
	}

	streamSnaps := streamSnapshotsOf(containerSnaps)
	if len(streamSnaps) != 1 {
		t.Fatalf(
			"alongside mode: want 1 kind=stream snapshot, got %d",
			len(streamSnaps),
		)
	}

	volumeSnaps := snapshotsByVolumeTag(containerSnaps)
	if len(volumeSnaps) != 1 {
		t.Fatalf(
			"alongside mode: want 1 volume snapshot, got %d (tags=%v)",
			len(volumeSnaps), allTags(containerSnaps),
		)
	}

	requireStdoutContains(t, backupResult, "Backed up "+containerName+" stream")
	requireStdoutContains(t, backupResult, "Backed up "+containerName+"/"+volumeName)
}

// TestPreBackup_OverrideContainer_RedirectsExecTarget verifies that
// `conba.pre-backup.container=<other>` causes the dump command to run in
// the override container instead of the labeled one. The override is the
// only container that can produce the unique payload, so a successful
// snapshot proves exec was redirected correctly.
func TestPreBackup_OverrideContainer_RedirectsExecTarget(t *testing.T) {
	labeledName := uniqueName(t, "conba-e2e-pb-labeled")
	adminName := uniqueName(t, "conba-e2e-pb-admin")
	volumeName := labeledName + "-data"
	adminPayload := "payload-from-admin-sidecar"

	// Admin sidecar carries no labels: it exists solely as the exec target.
	// Its uniquely identifiable hostname proves exec was redirected.
	startPreBackupContainer(t, preBackupContainerOpts{
		Name:       adminName,
		VolumeName: "",
		Labels:     nil,
		Command: []string{
			"sh", "-c",
			"echo " + adminPayload + " > /tmp/payload && tail -f /dev/null",
		},
	})

	startPreBackupContainer(t, preBackupContainerOpts{
		Name:       labeledName,
		VolumeName: volumeName,
		Command:    nil,
		Labels: map[string]string{
			"conba.pre-backup.command":   "cat /tmp/payload",
			"conba.pre-backup.container": adminName,
		},
	})

	cfg, repoPath := preBackupSetup(t, []string{"^" + labeledName + "$"}, true)

	backupResult := runConba(t, cfg, "backup")
	requireSuccess(t, backupResult, "conba backup")

	snaps := resticSnapshots(t, repoPath)
	streamSnap := requireSingleStreamSnapshot(t, snaps, labeledName)

	dumpBytes := resticDump(t, repoPath, streamSnap.ID, labeledName)

	got := strings.TrimRight(string(dumpBytes), "\n")
	if got != adminPayload {
		t.Fatalf(
			"override container: stored stream content = %q, want %q "+
				"(should match payload from admin sidecar, not labeled container)",
			got, adminPayload,
		)
	}
}

// TestPreBackup_FailedDumpCommand_FailsTargetCycleContinues verifies that
// a deliberately broken dump command fails the labeled target while
// allowing other targets in the cycle to complete, and that the cycle
// exits non-zero.
func TestPreBackup_FailedDumpCommand_FailsTargetCycleContinues(t *testing.T) {
	failingName := uniqueName(t, "conba-e2e-pb-fail")
	failingVolume := failingName + "-data"
	healthyName := uniqueName(t, "conba-e2e-pb-ok")
	healthyVolume := healthyName + "-data"

	startPreBackupContainer(t, preBackupContainerOpts{
		Name:       failingName,
		VolumeName: failingVolume,
		Command:    nil,
		Labels: map[string]string{
			"conba.pre-backup.command": "exit 1",
		},
	})

	startPreBackupContainer(t, preBackupContainerOpts{
		Name:       healthyName,
		VolumeName: healthyVolume,
		Command:    nil,
		Labels: map[string]string{
			"conba.pre-backup.command": "printf '" + preBackupDumpPayload + "'",
		},
	})

	includePattern := "^(" + failingName + "|" + healthyName + ")$"

	cfg, repoPath := preBackupSetup(t, []string{includePattern}, true)

	backupResult := runConba(t, cfg, "backup")
	if backupResult.Err != nil {
		t.Fatalf("conba backup: unexpected start error: %v", backupResult.Err)
	}

	if backupResult.ExitCode == 0 {
		t.Fatalf("conba backup exited 0, want non-zero; stdout=%q stderr=%q",
			backupResult.Stdout, backupResult.Stderr)
	}

	snaps := resticSnapshots(t, repoPath)

	failingSnaps := snapshotsForContainer(snaps, failingName)
	if len(failingSnaps) != 0 {
		t.Fatalf("failing container must not produce any snapshot; got %d (tags=%v)",
			len(failingSnaps), allTags(failingSnaps))
	}

	healthySnaps := snapshotsForContainer(snaps, healthyName)

	healthyStream := streamSnapshotsOf(healthySnaps)
	if len(healthyStream) != 1 {
		t.Fatalf("healthy container must still produce 1 stream snapshot; got %d (tags=%v)",
			len(healthyStream), allTags(healthySnaps))
	}

	requireStdoutContains(t, backupResult, "Failed "+failingName+" stream")
	requireStdoutContains(t, backupResult, "Backed up "+healthyName+" stream")
}

// TestPreBackup_DisabledFlag_LabelsIgnored verifies that with
// pre_backup_commands.enabled: false (the default), pre-backup labels are
// ignored entirely and the labeled container produces only volume
// snapshots, exactly as a non-labeled container would.
func TestPreBackup_DisabledFlag_LabelsIgnored(t *testing.T) {
	containerName := uniqueName(t, "conba-e2e-pb-disabled")
	volumeName := containerName + "-data"

	startPreBackupContainer(t, preBackupContainerOpts{
		Name:       containerName,
		VolumeName: volumeName,
		Command:    nil,
		Labels: map[string]string{
			"conba.pre-backup.command": "printf '" + preBackupDumpPayload + "'",
		},
	})

	cfg, repoPath := preBackupSetup(t, []string{"^" + containerName + "$"}, false)

	backupResult := runConba(t, cfg, "backup")
	requireSuccess(t, backupResult, "conba backup")

	snaps := resticSnapshots(t, repoPath)

	containerSnaps := snapshotsForContainer(snaps, containerName)
	if len(containerSnaps) != 1 {
		t.Fatalf(
			"disabled flag: want 1 volume snapshot for %s, got %d (tags=%v)",
			containerName, len(containerSnaps), allTags(containerSnaps),
		)
	}

	streamSnaps := streamSnapshotsOf(containerSnaps)
	if len(streamSnaps) != 0 {
		t.Fatalf(
			"disabled flag: want 0 kind=stream snapshots, got %d (tags=%v)",
			len(streamSnaps), allTags(streamSnaps),
		)
	}

	if !hasTagWithPrefix(containerSnaps[0].Tags, tagPrefixVolume) {
		t.Fatalf(
			"disabled flag: lone snapshot must carry volume= tag; tags=%v",
			containerSnaps[0].Tags,
		)
	}

	if strings.Contains(backupResult.Stdout, " stream") {
		t.Fatalf(
			"disabled flag: stdout must not mention stream; stdout=%q",
			backupResult.Stdout,
		)
	}
}

// TestPreBackup_DryRun_DoesNotInvokeDumpCommand verifies that --dry-run
// emits a "would" summary line for the pre-backup target and produces no
// snapshots in the repository.
func TestPreBackup_DryRun_DoesNotInvokeDumpCommand(t *testing.T) {
	containerName := uniqueName(t, "conba-e2e-pb-dryrun")
	volumeName := containerName + "-data"

	startPreBackupContainer(t, preBackupContainerOpts{
		Name:       containerName,
		VolumeName: volumeName,
		Command:    nil,
		Labels: map[string]string{
			"conba.pre-backup.command": "printf '" + preBackupDumpPayload + "'",
		},
	})

	cfg, repoPath := preBackupSetup(t, []string{"^" + containerName + "$"}, true)

	dryResult := runConba(t, cfg, "backup", "--dry-run")
	requireSuccess(t, dryResult, "conba backup --dry-run")

	requireStdoutContains(t, dryResult, "would be backed up")

	if !strings.Contains(dryResult.Stdout, containerName) {
		t.Fatalf(
			"dry-run: stdout must mention container %s; stdout=%q",
			containerName, dryResult.Stdout,
		)
	}

	// Spec acceptance: --dry-run with the feature enabled must announce the
	// pre-backup target via a "would run: <cmd> in <container>" line.
	wantRun := "would run: printf '" + preBackupDumpPayload + "' in " + containerName
	if !strings.Contains(dryResult.Stdout, wantRun) {
		t.Fatalf(
			"dry-run: stdout must contain %q; stdout=%q",
			wantRun, dryResult.Stdout,
		)
	}

	// Replace mode (the default) must mark the volume as replaced rather
	// than emitting the legacy "Would back up" listing for it. The legacy
	// listing surfaces the host source path, so its absence proves the
	// volume target was rendered as skipped.
	volumeSourceFragment := "/var/lib/docker/volumes/" + volumeName
	if strings.Contains(dryResult.Stdout, volumeSourceFragment) {
		t.Fatalf(
			"dry-run: replace mode must not emit the legacy volume listing for %s; "+
				"stdout=%q",
			volumeName, dryResult.Stdout,
		)
	}

	wantSkip := "would skip: " + containerName + "/" + volumeName
	if !strings.Contains(dryResult.Stdout, wantSkip) {
		t.Fatalf(
			"dry-run: stdout must contain %q; stdout=%q",
			wantSkip, dryResult.Stdout,
		)
	}

	snaps := resticSnapshots(t, repoPath)
	if len(snaps) != 0 {
		t.Fatalf(
			"dry-run: must produce 0 snapshots, got %d (tags=%v)",
			len(snaps), allTags(snaps),
		)
	}
}

// requireSingleStreamSnapshot fails the test unless containerName has
// exactly one snapshot tagged kind=stream in snaps. Justified for reuse
// across the replace and override-container scenarios.
func requireSingleStreamSnapshot(
	t *testing.T,
	snaps []ResticSnapshot,
	containerName string,
) ResticSnapshot {
	t.Helper()

	containerSnaps := snapshotsForContainer(snaps, containerName)
	if len(containerSnaps) != 1 {
		t.Fatalf(
			"want 1 snapshot for %s, got %d (tags=%v)",
			containerName, len(containerSnaps), allTags(snaps),
		)
	}

	streamSnaps := streamSnapshotsOf(containerSnaps)
	if len(streamSnaps) != 1 {
		t.Fatalf(
			"want 1 kind=stream snapshot for %s, got %d (tags=%v)",
			containerName, len(streamSnaps), allTags(containerSnaps),
		)
	}

	return streamSnaps[0]
}

// snapshotsByVolumeTag returns the subset of snaps that carry any volume=
// tag. Used by alongside-mode assertions to count plain volume snapshots
// alongside stream snapshots on the same container.
func snapshotsByVolumeTag(snaps []ResticSnapshot) []ResticSnapshot {
	var matches []ResticSnapshot

	for _, snap := range snaps {
		if hasTagWithPrefix(snap.Tags, tagPrefixVolume) {
			matches = append(matches, snap)
		}
	}

	return matches
}
