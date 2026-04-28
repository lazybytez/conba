//go:build e2e

package e2e_test

import (
	"path/filepath"
	"testing"
)

// TestVerify_FreshRepo asserts `conba verify` exits 0 on a freshly
// initialised repository.
func TestVerify_FreshRepo(t *testing.T) {
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
	requireSuccess(t, initResult, "conba init")

	verifyResult := runConba(t, cfg, "verify")
	requireSuccess(t, verifyResult, "conba verify")
	requireStdoutContains(t, verifyResult, "Repository verified.")
}

// TestVerify_FreshRepo_ReadData asserts `conba verify --read-data` exits 0
// on a freshly initialised repository.
func TestVerify_FreshRepo_ReadData(t *testing.T) {
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
	requireSuccess(t, initResult, "conba init")

	verifyResult := runConba(t, cfg, "verify", "--read-data")
	requireSuccess(t, verifyResult, "conba verify --read-data")
	requireStdoutContains(t, verifyResult, "Repository verified.")
}

// TestVerify_MissingRepo asserts `conba verify` against an uninitialised
// repository exits non-zero.
func TestVerify_MissingRepo(t *testing.T) {
	resetFixture(t)

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

	verifyResult := runConba(t, cfg, "verify")

	if verifyResult.Err != nil {
		t.Fatalf("conba verify: unexpected start error: %v", verifyResult.Err)
	}

	if verifyResult.ExitCode == 0 {
		t.Fatalf(
			"missing-repo: conba verify exited 0, want non-zero; stdout=%q stderr=%q",
			verifyResult.Stdout, verifyResult.Stderr,
		)
	}
}
