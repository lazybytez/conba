//go:build e2e

package e2e_test

import (
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
)

// TestInit_FreshRepo asserts that `conba init` creates the restic repo on
// disk and that a follow-up `conba status` reports the ready state.
func TestInit_FreshRepo(t *testing.T) {
	resetFixture(t)

	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")

	writeConfig(t, dir, configOpts{
		ResticRepoPath:           repoPath,
		ResticPassword:           "",
		IncludeNames:             nil,
		IncludeNamePatterns:      nil,
		ExcludeNames:             nil,
		ResticEnvironment:        nil,
		PreBackupCommandsEnabled: false,
	})

	cfg := runConfig{Dir: dir, Stdin: nil, Env: nil}

	initResult := runConba(t, cfg, "init")
	requireSuccess(t, initResult, "conba init")

	configPath := filepath.Join(repoPath, "config")

	_, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("expected restic config at %q after init: %v", configPath, err)
	}

	statusResult := runConba(t, cfg, "status")
	requireSuccess(t, statusResult, "conba status")
	requireStdoutContains(t, statusResult, "Status:     ready")
	requireStdoutContains(t, statusResult, repoPath)
}

// TestInit_AlreadyInitialized asserts that a second `conba init` is an
// idempotent no-op: exit 0 and the on-disk config file is byte-identical
// before and after. Idempotence is deliberate: the restic client swallows
// the "already initialized" / "config file already exists" stderr family.
func TestInit_AlreadyInitialized(t *testing.T) {
	resetFixture(t)

	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")

	writeConfig(t, dir, configOpts{
		ResticRepoPath:           repoPath,
		ResticPassword:           "",
		IncludeNames:             nil,
		IncludeNamePatterns:      nil,
		ExcludeNames:             nil,
		ResticEnvironment:        nil,
		PreBackupCommandsEnabled: false,
	})

	cfg := runConfig{Dir: dir, Stdin: nil, Env: nil}

	first := runConba(t, cfg, "init")
	requireSuccess(t, first, "first conba init")

	configPath := filepath.Join(repoPath, "config")
	beforeBytes := readFile(t, configPath)
	beforeHash := sha256.Sum256(beforeBytes)

	second := runConba(t, cfg, "init")
	requireSuccess(t, second, "second conba init")

	afterBytes := readFile(t, configPath)
	afterHash := sha256.Sum256(afterBytes)

	if !bytes.Equal(beforeHash[:], afterHash[:]) {
		t.Fatalf(
			"restic config file bytes changed across idempotent init; before=%x after=%x",
			beforeHash, afterHash,
		)
	}
}
