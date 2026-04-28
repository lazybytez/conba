//go:build e2e

package e2e_test

import (
	"context"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// restoreVolumePayload is the deterministic file content written into a
// labeled container's volume before backup. The volume restore scenarios
// assert the restored tree contains a file with this exact value.
const restoreVolumePayload = "conba-e2e-restore-volume-payload"

// restoreStreamPayload is the deterministic stdout produced by stream-mode
// dump commands so restore scenarios can assert byte-equality of what the
// in-container restore command receives.
const restoreStreamPayload = "conba-e2e-restore-stream-payload"

// streamRestoreSetup starts a labeled stream-source container, takes a
// backup, and returns the runConfig + repo path the restore call needs.
// Reused across the stream-mode scenarios to keep each test focused on
// the assertion rather than fixture wiring.
func streamRestoreSetup(
	t *testing.T,
	containerName string,
	extraLabels map[string]string,
) (runConfig, string) {
	t.Helper()

	labels := map[string]string{
		"conba.pre-backup.command": "printf '" + restoreStreamPayload + "'",
	}

	maps.Copy(labels, extraLabels)

	// Discovery only sees containers with at least one eligible mount, so
	// give the container a named volume even though the stream snapshot
	// (replace mode) replaces the volume backup.
	startPreBackupContainer(t, preBackupContainerOpts{
		Name:       containerName,
		VolumeName: containerName + "-data",
		Command:    nil,
		Labels:     labels,
	})

	cfg, repoPath := preBackupSetup(t, []string{"^" + containerName + "$"}, true)

	backupResult := runConba(t, cfg, "backup")
	requireSuccess(t, backupResult, "conba backup")

	return cfg, repoPath
}

// volumeRestoreSetup starts an ad-hoc container with a named volume,
// populates it with restoreVolumePayload, takes a backup, then returns
// the runConfig, container name, and volume name. Scenarios 1-3 and the
// volume side of 9 reuse this to share the backup boilerplate.
func volumeRestoreSetup(t *testing.T) (runConfig, string, string) {
	t.Helper()

	containerName := uniqueName(t, "conba-e2e-restore-vol")
	volumeName := containerName + "-data"

	startPreBackupContainer(t, preBackupContainerOpts{
		Name:       containerName,
		VolumeName: volumeName,
		Command:    nil,
		Labels:     nil,
	})

	dockerExec(t, containerName,
		"sh", "-c",
		"echo -n "+restoreVolumePayload+" > /data/payload.txt",
	)

	cfg, _ := preBackupSetup(t, []string{"^" + containerName + "$"}, false)

	backupResult := runConba(t, cfg, "backup")
	requireSuccess(t, backupResult, "conba backup")

	return cfg, containerName, volumeName
}

// dockerExec runs `docker exec -i <name> <args...>` for ad-hoc containers
// that were started outside docker compose (where composeExec does not
// apply). Used by both the volume and stream scenarios to seed and read
// in-container state.
func dockerExec(t *testing.T, name string, args ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()

	argv := append([]string{"exec", "-i", name}, args...)
	cmd := exec.CommandContext(ctx, "docker", argv...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker exec %s %v: %v: %s",
			name, args, err, strings.TrimSpace(string(out)))
	}

	return string(out)
}

// restoredVolumeFile returns the path inside targetDir where a restored
// volume snapshot stores filename. conba backs up the docker volume host
// path /var/lib/docker/volumes/<vol>/_data, so restic re-creates that
// tree under --target.
func restoredVolumeFile(targetDir, volumeName, filename string) string {
	return filepath.Join(
		targetDir, "var", "lib", "docker", "volumes", volumeName, "_data", filename,
	)
}

// TestRestore_VolumeMode_HappyPath asserts that a volume restore writes
// the snapshot's payload to the requested target path even after the
// live volume has diverged.
func TestRestore_VolumeMode_HappyPath(t *testing.T) {
	cfg, containerName, volumeName := volumeRestoreSetup(t)

	dockerExec(t, containerName,
		"sh", "-c",
		"echo -n different-content > /data/payload.txt",
	)

	target := filepath.Join(cfg.Dir, "restore-target")

	restoreResult := runConba(t, cfg, "restore",
		"--container", containerName,
		"--volume", volumeName,
		"--to", target,
	)
	requireSuccess(t, restoreResult, "conba restore (volume happy path)")

	got := readFile(t, restoredVolumeFile(target, volumeName, "payload.txt"))
	if string(got) != restoreVolumePayload {
		t.Fatalf("restored payload = %q, want %q", string(got), restoreVolumePayload)
	}
}

// TestRestore_VolumeMode_RefusesNonEmptyDestinationWithoutForce asserts
// that conba refuses to overwrite an occupied --to without --force, exits
// non-zero with a "destination not empty" message, and leaves the
// pre-existing file untouched.
func TestRestore_VolumeMode_RefusesNonEmptyDestinationWithoutForce(t *testing.T) {
	cfg, containerName, volumeName := volumeRestoreSetup(t)

	target := filepath.Join(cfg.Dir, "restore-target")

	err := os.MkdirAll(target, 0o700)
	if err != nil {
		t.Fatalf("mkdir target: %v", err)
	}

	preExisting := filepath.Join(target, "do-not-overwrite.txt")
	preContent := []byte("pre-existing content")

	err = os.WriteFile(preExisting, preContent, 0o600)
	if err != nil {
		t.Fatalf("write pre-existing file: %v", err)
	}

	restoreResult := runConba(t, cfg, "restore",
		"--container", containerName,
		"--volume", volumeName,
		"--to", target,
	)
	if restoreResult.Err != nil {
		t.Fatalf("conba restore: unexpected start error: %v", restoreResult.Err)
	}

	if restoreResult.ExitCode == 0 {
		t.Fatalf(
			"conba restore exited 0, want non-zero; stdout=%q stderr=%q",
			restoreResult.Stdout, restoreResult.Stderr,
		)
	}

	combined := restoreResult.Stdout + restoreResult.Stderr
	if !strings.Contains(combined, "destination") || !strings.Contains(combined, "not empty") {
		t.Fatalf(
			"want stdout/stderr to mention destination not empty; stdout=%q stderr=%q",
			restoreResult.Stdout, restoreResult.Stderr,
		)
	}

	got := readFile(t, preExisting)
	if string(got) != string(preContent) {
		t.Fatalf("pre-existing file modified: got %q, want %q", string(got), preContent)
	}
}

// TestRestore_VolumeMode_ForceOverwritesNonEmptyDestination asserts that
// --force lets the same scenario succeed: the snapshot's payload appears
// in the destination after the run.
func TestRestore_VolumeMode_ForceOverwritesNonEmptyDestination(t *testing.T) {
	cfg, containerName, volumeName := volumeRestoreSetup(t)

	target := filepath.Join(cfg.Dir, "restore-target")

	err := os.MkdirAll(target, 0o700)
	if err != nil {
		t.Fatalf("mkdir target: %v", err)
	}

	stale := filepath.Join(target, "stale.txt")

	err = os.WriteFile(stale, []byte("stale"), 0o600)
	if err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	restoreResult := runConba(t, cfg, "restore",
		"--container", containerName,
		"--volume", volumeName,
		"--to", target,
		"--force",
	)
	requireSuccess(t, restoreResult, "conba restore --force")

	got := readFile(t, restoredVolumeFile(target, volumeName, "payload.txt"))
	if string(got) != restoreVolumePayload {
		t.Fatalf("restored payload = %q, want %q", string(got), restoreVolumePayload)
	}
}

// TestRestore_StreamMode_ToCommandFlagPipesPayload asserts that a stream
// snapshot restored with --to-command pipes the snapshot's stored bytes
// into the command running inside the target container, leaving the
// expected file behind.
func TestRestore_StreamMode_ToCommandFlagPipesPayload(t *testing.T) {
	containerName := uniqueName(t, "conba-e2e-restore-stream")

	cfg, _ := streamRestoreSetup(t, containerName, nil)

	restoreResult := runConba(t, cfg, "restore",
		"--container", containerName,
		"--to-command", "tee /tmp/proof",
	)
	requireSuccess(t, restoreResult, "conba restore --to-command")

	got := dockerExec(t, containerName, "cat", "/tmp/proof")
	if got != restoreStreamPayload {
		t.Fatalf("/tmp/proof = %q, want %q", got, restoreStreamPayload)
	}
}

// TestRestore_StreamMode_RestoreCommandLabelUsedWhenFlagAbsent asserts
// that a container carrying conba.pre-backup.restore-command supplies
// the in-container command to conba restore when --to-command is not
// passed.
func TestRestore_StreamMode_RestoreCommandLabelUsedWhenFlagAbsent(t *testing.T) {
	containerName := uniqueName(t, "conba-e2e-restore-stream-label")

	cfg, _ := streamRestoreSetup(t, containerName, map[string]string{
		"conba.pre-backup.restore-command": "tee /tmp/proof-from-label",
	})

	restoreResult := runConba(t, cfg, "restore",
		"--container", containerName,
	)
	requireSuccess(t, restoreResult, "conba restore (label-driven)")

	got := dockerExec(t, containerName, "cat", "/tmp/proof-from-label")
	if got != restoreStreamPayload {
		t.Fatalf("/tmp/proof-from-label = %q, want %q", got, restoreStreamPayload)
	}
}

// TestRestore_StreamMode_FlagWinsOverLabel asserts that --to-command
// overrides the conba.pre-backup.restore-command label: the flag's
// destination receives the payload while the label's destination does
// not.
func TestRestore_StreamMode_FlagWinsOverLabel(t *testing.T) {
	containerName := uniqueName(t, "conba-e2e-restore-stream-both")

	cfg, _ := streamRestoreSetup(t, containerName, map[string]string{
		"conba.pre-backup.restore-command": "tee /tmp/from-label",
	})

	restoreResult := runConba(t, cfg, "restore",
		"--container", containerName,
		"--to-command", "tee /tmp/from-flag",
	)
	requireSuccess(t, restoreResult, "conba restore (flag wins)")

	got := dockerExec(t, containerName, "cat", "/tmp/from-flag")
	if got != restoreStreamPayload {
		t.Fatalf("/tmp/from-flag = %q, want %q", got, restoreStreamPayload)
	}

	// The label's path must not exist; if it did, both commands ran.
	checkResult := dockerExecAllowFail(t, containerName,
		"sh", "-c", "test ! -e /tmp/from-label",
	)
	if checkResult.ExitCode != 0 {
		t.Fatalf(
			"label's destination was written too: flag did not win; stdout=%q",
			checkResult.Stdout,
		)
	}
}

// TestRestore_StreamMode_RefusesWhenContainerNotRunning stops the labeled
// container after the backup and asserts the restore exits non-zero with
// a "not running" message instead of attempting docker exec.
func TestRestore_StreamMode_RefusesWhenContainerNotRunning(t *testing.T) {
	containerName := uniqueName(t, "conba-e2e-restore-stream-stopped")

	cfg, _ := streamRestoreSetup(t, containerName, nil)

	stopContainer(t, containerName)

	restoreResult := runConba(t, cfg, "restore",
		"--container", containerName,
		"--to-command", "tee /tmp/proof",
	)
	if restoreResult.Err != nil {
		t.Fatalf("conba restore: unexpected start error: %v", restoreResult.Err)
	}

	if restoreResult.ExitCode == 0 {
		t.Fatalf(
			"conba restore exited 0 against a stopped container; stdout=%q stderr=%q",
			restoreResult.Stdout, restoreResult.Stderr,
		)
	}

	combined := restoreResult.Stdout + restoreResult.Stderr
	if !strings.Contains(combined, "not running") {
		t.Fatalf(
			"want stdout/stderr to mention 'not running'; stdout=%q stderr=%q",
			restoreResult.Stdout, restoreResult.Stderr,
		)
	}
}

// TestRestore_StreamMode_RefusesWithoutFlagOrLabel asserts that a stream
// snapshot with neither --to-command nor a restore-command label fails
// with a clear error mentioning both options.
func TestRestore_StreamMode_RefusesWithoutFlagOrLabel(t *testing.T) {
	containerName := uniqueName(t, "conba-e2e-restore-stream-nocmd")

	cfg, _ := streamRestoreSetup(t, containerName, nil)

	restoreResult := runConba(t, cfg, "restore",
		"--container", containerName,
	)
	if restoreResult.Err != nil {
		t.Fatalf("conba restore: unexpected start error: %v", restoreResult.Err)
	}

	if restoreResult.ExitCode == 0 {
		t.Fatalf(
			"conba restore exited 0 with no command source; stdout=%q stderr=%q",
			restoreResult.Stdout, restoreResult.Stderr,
		)
	}

	combined := restoreResult.Stdout + restoreResult.Stderr
	if !strings.Contains(combined, "--to-command") {
		t.Fatalf(
			"want error to mention --to-command; stdout=%q stderr=%q",
			restoreResult.Stdout, restoreResult.Stderr,
		)
	}

	if !strings.Contains(combined, "restore-command") {
		t.Fatalf(
			"want error to mention restore-command label; stdout=%q stderr=%q",
			restoreResult.Stdout, restoreResult.Stderr,
		)
	}
}

// TestRestore_VolumeMode_DryRunWritesNoFiles asserts that --dry-run for
// volume mode prints "would restore..." and produces no files at the
// target path.
func TestRestore_VolumeMode_DryRunWritesNoFiles(t *testing.T) {
	cfg, containerName, volumeName := volumeRestoreSetup(t)

	target := filepath.Join(cfg.Dir, "restore-target")

	restoreResult := runConba(t, cfg, "restore",
		"--container", containerName,
		"--volume", volumeName,
		"--to", target,
		"--dry-run",
	)
	requireSuccess(t, restoreResult, "conba restore --dry-run (volume)")

	requireStdoutContains(t, restoreResult, "would restore")
	requireStdoutContains(t, restoreResult, target)

	_, err := os.Stat(target)
	if err == nil {
		entries, readErr := os.ReadDir(target)
		if readErr != nil {
			t.Fatalf("read dry-run target dir %q: %v", target, readErr)
		}

		if len(entries) != 0 {
			t.Fatalf("dry-run target %q must be empty, got %d entries", target, len(entries))
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat dry-run target %q: %v", target, err)
	}
}

// TestRestore_StreamMode_DryRunDoesNotMutateContainer asserts that
// --dry-run for stream mode prints "would restore..." and does not
// pipe the snapshot payload into the container: the file the
// restore-command would have written must not contain the payload.
func TestRestore_StreamMode_DryRunDoesNotMutateContainer(t *testing.T) {
	containerName := uniqueName(t, "conba-e2e-restore-stream-dry")

	cfg, _ := streamRestoreSetup(t, containerName, nil)

	restoreResult := runConba(t, cfg, "restore",
		"--container", containerName,
		"--to-command", "tee /tmp/dryrun-proof",
		"--dry-run",
	)
	requireSuccess(t, restoreResult, "conba restore --dry-run (stream)")

	requireStdoutContains(t, restoreResult, "would restore")
	requireStdoutContains(t, restoreResult, containerName)

	// Read the file the live run would have populated. The dry run must
	// not deliver the payload to it -- absence or any content other than
	// restoreStreamPayload counts as the container being unchanged.
	got := dockerExecAllowFail(t, containerName, "cat", "/tmp/dryrun-proof")
	if got.ExitCode == 0 && got.Stdout == restoreStreamPayload {
		t.Fatalf(
			"dry-run delivered payload to container; /tmp/dryrun-proof = %q",
			got.Stdout,
		)
	}
}

// dockerExecResult is the captured outcome of dockerExecAllowFail. Used by
// scenarios that need to assert specific exit codes or absence semantics
// (e.g. "this file was NOT created").
type dockerExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// dockerExecAllowFail runs `docker exec -i <name> <args...>` and returns
// the captured stdout/stderr/exit code without failing the test. Used by
// scenarios that assert non-zero exit (e.g. `test ! -e <path>`) or the
// absence of a file (cat returns 1 for missing).
func dockerExecAllowFail(t *testing.T, name string, args ...string) dockerExecResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()

	argv := append([]string{"exec", "-i", name}, args...)
	cmd := exec.CommandContext(ctx, "docker", argv...)

	var stdout, stderr strings.Builder

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := dockerExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err == nil {
		return result
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()

		return result
	}

	t.Fatalf("docker exec %s %v: %v", name, args, err)

	return result
}

// stopContainer issues `docker stop <name>` and fails the test on error.
// Used by scenario 7 to put the labeled container into a non-running state
// between backup and restore.
func stopContainer(t *testing.T, name string) {
	t.Helper()

	runDocker(t, "stop", name)
}
