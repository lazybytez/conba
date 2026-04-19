//go:build e2e

package e2e_test

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

// TestInit_FreshRepo verifies that `conba init` against an empty repo path
// creates the restic repository on disk and that a follow-up `conba status`
// reports the ready state. The expected "ready" line text is emitted by
// printStatus in internal/cli/status.go.
//
//nolint:paralleltest // Suite runs with -p 1; t.Parallel() is forbidden.
func TestInit_FreshRepo(t *testing.T) {
	resetFixture(t)

	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")

	writeConfig(t, dir, configOpts{
		ResticRepoPath:      repoPath,
		ResticPassword:      "",
		IncludeNames:        nil,
		IncludeNamePatterns: nil,
		ExcludeNames:        nil,
	})

	cfg := runConfig{Dir: dir, Stdin: nil, Env: nil}

	initResult := runConba(t, cfg, "init")
	requireExit(t, initResult, "conba init", 0)

	configPath := filepath.Join(repoPath, "config")

	_, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("expected restic config at %q after init: %v", configPath, err)
	}

	statusResult := runConba(t, cfg, "status")
	requireExit(t, statusResult, "conba status", 0)
	requireStdoutContains(t, statusResult, "Status:     ready")
	requireStdoutContains(t, statusResult, repoPath)
}

// TestInit_AlreadyInitialized runs `conba init` twice in a row and asserts
// the second invocation is an idempotent no-op:
//   - exit code 0 (Client.Init in internal/restic/init.go deliberately
//     swallows the "already initialized" / "config file already exists"
//     restic stderr patterns and returns nil).
//   - the on-disk `config` file is byte-identical before and after the
//     second run.
//
// Note: the task plan originally specified a non-zero exit + stderr match
// against an "already initialized" message. That contradicts the current
// Client.Init implementation, which deliberately returns nil for that
// stderr family to preserve idempotence.
//
//nolint:paralleltest // Suite runs with -p 1; t.Parallel() is forbidden.
func TestInit_AlreadyInitialized(t *testing.T) {
	resetFixture(t)

	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")

	writeConfig(t, dir, configOpts{
		ResticRepoPath:      repoPath,
		ResticPassword:      "",
		IncludeNames:        nil,
		IncludeNamePatterns: nil,
		ExcludeNames:        nil,
	})

	cfg := runConfig{Dir: dir, Stdin: nil, Env: nil}

	first := runConba(t, cfg, "init")
	requireExit(t, first, "first conba init", 0)

	configPath := filepath.Join(repoPath, "config")
	beforeBytes := readFile(t, configPath)
	beforeHash := sha256.Sum256(beforeBytes)

	second := runConba(t, cfg, "init")
	requireExit(t, second, "second conba init", 0)

	afterBytes := readFile(t, configPath)
	afterHash := sha256.Sum256(afterBytes)

	if !bytes.Equal(beforeHash[:], afterHash[:]) {
		t.Fatalf(
			"restic config file bytes changed across idempotent init; before=%x after=%x",
			beforeHash, afterHash,
		)
	}
}
