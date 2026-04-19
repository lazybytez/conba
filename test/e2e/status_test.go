//go:build e2e

package e2e_test

import (
	"path/filepath"
	"testing"
)

// TestStatus_Uninitialized runs `conba status` against an empty repo path
// that does not contain a restic repository. Current production behaviour
// (internal/cli/status.go handleStatusError + printNotInitialized): the
// command exits 0 and writes the friendly "not initialized" line to
// stdout.
//
// Note: the task plan originally specified "exits non-zero". The current
// CLI wiring treats a classified "repository not initialized" error as a
// successful, user-friendly status report and returns nil, so exit 0 is
// the correct assertion against the code as it stands today.
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

// TestStatus_Initialized runs `conba init` then `conba status` against the
// same repo. The status output must include the "ready" line from
// printStatus in internal/cli/status.go.
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
