//go:build e2e

package e2e_test

import (
	"path/filepath"
	"testing"
)

// TestUnlock_FreshRepo asserts `conba unlock` exits 0 on a freshly
// initialised repository. restic unlock is idempotent: removing zero stale
// locks still succeeds, so a clean repo is the simplest success case.
func TestUnlock_FreshRepo(t *testing.T) {
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
	requireSuccess(t, initResult, "conba init")

	unlockResult := runConba(t, cfg, "unlock")
	requireSuccess(t, unlockResult, "conba unlock")
	requireStdoutContains(t, unlockResult, "Repository unlocked.")
}

// TestUnlock_MissingRepo asserts `conba unlock` against an uninitialised
// repository exits non-zero: restic cannot open a repository that does not
// exist, and conba surfaces that as a failure rather than a clean no-op.
func TestUnlock_MissingRepo(t *testing.T) {
	dir := t.TempDir()
	repoPath := filepath.Join(dir, "missing")

	writeConfig(t, dir, configOpts{
		ResticRepoPath:      repoPath,
		ResticPassword:      "",
		IncludeNames:        nil,
		IncludeNamePatterns: nil,
		ExcludeNames:        nil,
	})

	cfg := runConfig{Dir: dir, Stdin: nil, Env: nil}

	unlockResult := runConba(t, cfg, "unlock")

	if unlockResult.Err != nil {
		t.Fatalf("conba unlock: unexpected start error: %v", unlockResult.Err)
	}

	if unlockResult.ExitCode == 0 {
		t.Fatalf(
			"missing-repo: conba unlock exited 0, want non-zero; stdout=%q stderr=%q",
			unlockResult.Stdout, unlockResult.Stderr,
		)
	}
}
