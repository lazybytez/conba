//go:build e2e

package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// decodeEvents parses NDJSON output into a slice of generic event objects.
func decodeEvents(t *testing.T, out string) []map[string]any {
	t.Helper()

	var events []map[string]any

	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		event := map[string]any{}

		err := json.Unmarshal([]byte(line), &event)
		if err != nil {
			t.Fatalf("output line is not JSON (%v): %q", err, line)
		}

		events = append(events, event)
	}

	return events
}

// requireEvent fails the test unless out contains an NDJSON object whose
// "event" field equals name, returning that event.
func requireEvent(t *testing.T, out, name string) map[string]any {
	t.Helper()

	for _, event := range decodeEvents(t, out) {
		if event["event"] == name {
			return event
		}
	}

	t.Fatalf("no %q event in output:\n%s", name, out)

	return nil
}

// TestOutput_JSONMode_EmitsEvents drives the read and write commands with
// --output json and asserts each emits its defined NDJSON events.
func TestOutput_JSONMode_EmitsEvents(t *testing.T) {
	resetFixture(t)

	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")

	writeConfig(t, dir, configOpts{ResticRepoPath: repoPath})

	cfg := runConfig{Dir: dir, Stdin: nil, Env: nil}

	initRes := runConba(t, cfg, "init", "--output", "json")
	requireSuccess(t, initRes, "conba init --output json")
	requireEvent(t, initRes.Stdout, "init.done")

	backupRes := runConba(t, cfg, "backup", "--output", "json")
	requireSuccess(t, backupRes, "conba backup --output json")
	requireEvent(t, backupRes.Stdout, "backup.target")
	requireEvent(t, backupRes.Stdout, "backup.summary")

	snapsRes := runConba(t, cfg, "snapshots", "--output", "json")
	requireSuccess(t, snapsRes, "conba snapshots --output json")
	requireEvent(t, snapsRes.Stdout, "snapshot")
	requireEvent(t, snapsRes.Stdout, "snapshots.summary")

	statusRes := runConba(t, cfg, "status", "--output", "json")
	requireSuccess(t, statusRes, "conba status --output json")

	status := requireEvent(t, statusRes.Stdout, "repo.status")
	if status["state"] != "ok" {
		t.Errorf("repo.status state = %v, want ok", status["state"])
	}

	verifyRes := runConba(t, cfg, "verify", "--output", "json")
	requireSuccess(t, verifyRes, "conba verify --output json")
	requireEvent(t, verifyRes.Stdout, "verify.done")
}

// TestOutput_ExitCode_InvalidConfigIsOne asserts a config/precondition
// error maps to exit code 1.
func TestOutput_ExitCode_InvalidConfigIsOne(t *testing.T) {
	resetFixture(t)

	dir := t.TempDir()
	writeConfig(t, dir, configOpts{ResticRepoPath: filepath.Join(dir, "repo")})

	cfg := runConfig{Dir: dir, Stdin: nil, Env: []string{"CONBA_OUTPUT_FORMAT=bogus"}}

	res := runConba(t, cfg, "status")
	requireFailure(t, res, "conba status")

	if res.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1 (precondition)", res.ExitCode)
	}
}

// TestOutput_ExitCode_TotalFailureIsThree asserts a cycle where every
// target fails maps to exit code 3.
func TestOutput_ExitCode_TotalFailureIsThree(t *testing.T) {
	failingName := uniqueName(t, "conba-e2e-total-fail")

	startPreBackupContainer(t, preBackupContainerOpts{
		Name:       failingName,
		VolumeName: failingName + "-data",
		Labels: map[string]string{
			"conba.pre-backup.command": "exit 1",
		},
	})

	cfg, _ := preBackupSetup(t, []string{"^" + failingName + "$"}, true)

	res := runConba(t, cfg, "backup")
	requireFailure(t, res, "conba backup")

	if res.ExitCode != 3 {
		t.Errorf("exit code = %d, want 3 (total failure)", res.ExitCode)
	}
}

// TestOutput_NoSecretInOutput asserts the configured repository password
// never appears in command output, in either render mode.
func TestOutput_NoSecretInOutput(t *testing.T) {
	resetFixture(t)

	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")
	cacheDir := filepath.Join(dir, "restic-cache")

	err := os.MkdirAll(cacheDir, 0o700)
	if err != nil {
		t.Fatalf("create restic cache dir: %v", err)
	}

	const (
		passwordSecret = "super-secret-pw-9z8y7x"
		envSecret      = "super-secret-env-4a5b6c"
	)

	// Set a restic.environment value too, so the assertion guards the env
	// surface (e.g. an S3 secret key) as well as the password. PATH and the
	// cache dir are required because a non-empty environment block replaces
	// the inherited process environment for the restic subprocess.
	writeConfig(t, dir, configOpts{
		ResticRepoPath: repoPath,
		ResticPassword: passwordSecret,
		ResticEnvironment: map[string]string{
			"PATH":                  os.Getenv("PATH"),
			"RESTIC_CACHE_DIR":      cacheDir,
			"AWS_SECRET_ACCESS_KEY": envSecret,
		},
	})

	cfg := runConfig{Dir: dir, Stdin: nil, Env: nil}

	invocations := [][]string{
		{"init"},
		{"backup"},
		{"snapshots"},
		{"status"},
		{"verify"},
		{"backup", "--output", "json"},
		{"status", "--output", "json"},
	}

	for _, args := range invocations {
		res := runConba(t, cfg, args...)
		combined := res.Stdout + res.Stderr

		for _, leaked := range []string{passwordSecret, envSecret} {
			if strings.Contains(combined, leaked) {
				t.Errorf("conba %v leaked a secret (%q) in output:\nstdout=%s\nstderr=%s",
					args, leaked, res.Stdout, res.Stderr)
			}
		}
	}
}
