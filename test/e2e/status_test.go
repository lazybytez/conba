//go:build e2e

package e2e_test

import (
	"path/filepath"
	"testing"
)

// TestStatus_Uninitialized asserts that `conba status` against a missing
// repo exits 0 and prints the friendly "not initialized" line. The CLI
// classifies "repository not initialized" as a reportable status rather
// than an error, so exit 0 is intentional.
func TestStatus_Uninitialized(t *testing.T) {
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

	result := runConba(t, cfg, "status")
	requireSuccess(t, result, "conba status")
	requireStdoutContains(t, result, "not initialized (run 'conba init' to create)")
	requireStdoutContains(t, result, repoPath)
}

// TestStatus_Initialized asserts that `conba status` reports the ready
// state against a freshly initialized repo.
func TestStatus_Initialized(t *testing.T) {
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

	result := runConba(t, cfg, "status")
	requireSuccess(t, result, "conba status")
	requireStdoutContains(t, result, "Status:     ready")
	requireStdoutContains(t, result, repoPath)
}
