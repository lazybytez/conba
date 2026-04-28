//go:build e2e

package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestDiff_BetweenSnapshots takes two backups of the app container with a
// file change between them and asserts `conba diff <id1> <id2>` reports
// the new file.
func TestDiff_BetweenSnapshots(t *testing.T) {
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

	first := runConba(t, cfg, "backup")
	requireSuccess(t, first, "first conba backup")

	composeExec(t, containerApp, nil,
		"sh", "-c", "echo diff-payload > /data/diff-marker.txt")

	second := runConba(t, cfg, "backup")
	requireSuccess(t, second, "second conba backup")

	appSnaps := snapshotsForContainer(resticSnapshots(t, repoPath), containerApp)
	if len(appSnaps) < 2 {
		t.Fatalf("want >=2 app snapshots, got %d", len(appSnaps))
	}

	// Diff the two newest snapshots for the container. Indexing from the
	// end is robust to additional snapshots from earlier setup or future
	// changes to fixture priming.
	older := appSnaps[len(appSnaps)-2]
	newer := appSnaps[len(appSnaps)-1]

	diffResult := runConba(t, cfg, "diff", older.ID, newer.ID)
	requireSuccess(t, diffResult, "conba diff")

	if !strings.Contains(diffResult.Stdout, "diff-marker.txt") {
		t.Fatalf(
			"expected diff to mention diff-marker.txt; stdout=%q",
			diffResult.Stdout,
		)
	}
}

// TestDiff_MissingSnapshot asserts `conba diff` against an unknown
// snapshot ID exits non-zero.
func TestDiff_MissingSnapshot(t *testing.T) {
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

	diffResult := runConba(t, cfg, "diff", "nonexistent1", "nonexistent2")

	if diffResult.Err != nil {
		t.Fatalf("conba diff: unexpected start error: %v", diffResult.Err)
	}

	if diffResult.ExitCode == 0 {
		t.Fatalf(
			"missing-snapshot: conba diff exited 0, want non-zero; stdout=%q stderr=%q",
			diffResult.Stdout, diffResult.Stderr,
		)
	}
}
