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
//
//nolint:paralleltest // Suite runs with -p 1; t.Parallel() is forbidden.
func TestStatus_Uninitialized(t *testing.T) {
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

	result := runConba(t, cfg, "status")
	requireExit(t, result, "conba status", 0)
	requireStdoutContains(t, result, "not initialized (run 'conba init' to create)")
	requireStdoutContains(t, result, repoPath)
}

// TestStatus_Initialized asserts that `conba status` reports the ready
// state against a freshly initialized repo.
//
//nolint:paralleltest // Suite runs with -p 1; t.Parallel() is forbidden.
func TestStatus_Initialized(t *testing.T) {
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

	result := runConba(t, cfg, "status")
	requireExit(t, result, "conba status", 0)
	requireStdoutContains(t, result, "Status:     ready")
	requireStdoutContains(t, result, repoPath)
}
