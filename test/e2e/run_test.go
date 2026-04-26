//go:build e2e

package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/config"
)

// TestRun_FreshRepo_FullCycle verifies that `conba run` against a non-existent
// repo executes init + backup + forget in one shot: the repo is initialised,
// at least one snapshot is created, and the global retention policy is
// applied (keep_daily=1 collapses snapshots that would otherwise share a day).
func TestRun_FreshRepo_FullCycle(t *testing.T) {
	resetFixture(t)

	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")

	writeConfig(t, dir, configOpts{
		ResticRepoPath:      repoPath,
		ResticPassword:      "",
		IncludeNames:        nil,
		IncludeNamePatterns: nil,
		ExcludeNames:        nil,
		Retention:           config.RetentionConfig{KeepDaily: 1},
	})

	cfg := runConfig{Dir: dir, Stdin: nil, Env: nil}

	result := runConba(t, cfg, "run")
	requireSuccess(t, result, "conba run")

	requireStdoutContains(t, result, "==> init")
	requireStdoutContains(t, result, "==> backup")
	requireStdoutContains(t, result, "==> forget")

	configPath := filepath.Join(repoPath, "config")

	_, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("expected restic config at %q after run: %v", configPath, err)
	}

	snaps := resticSnapshots(t, repoPath)
	if len(snaps) == 0 {
		t.Fatalf("expected at least one snapshot after `conba run`; got none")
	}

	requireAtLeastOneSnapshot(t, snaps, containerApp)
	requireAtLeastOneSnapshot(t, snaps, containerMySQL)
}

// TestRun_AlreadyInitialized_Idempotent verifies that running `conba run` a
// second time on an already-initialised repo exits 0. The "repository already
// initialized" condition must be swallowed by the init phase rather than
// surfacing as an error. With keep_daily=1 the second run also exercises the
// forget phase against an existing repo.
func TestRun_AlreadyInitialized_Idempotent(t *testing.T) {
	resetFixture(t)

	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")

	writeConfig(t, dir, configOpts{
		ResticRepoPath:      repoPath,
		ResticPassword:      "",
		IncludeNames:        nil,
		IncludeNamePatterns: nil,
		ExcludeNames:        nil,
		Retention:           config.RetentionConfig{KeepDaily: 1},
	})

	cfg := runConfig{Dir: dir, Stdin: nil, Env: nil}

	first := runConba(t, cfg, "run")
	requireSuccess(t, first, "first conba run")

	second := runConba(t, cfg, "run")
	requireSuccess(t, second, "second conba run")

	requireStdoutContains(t, second, "==> init")
	requireStdoutContains(t, second, "==> backup")
	requireStdoutContains(t, second, "==> forget")

	snaps := resticSnapshots(t, repoPath)
	if len(snaps) == 0 {
		t.Fatalf("expected at least one snapshot after two `conba run` invocations; got none")
	}
}

// TestRun_NoForget_SkipsForgetPhase verifies that `--no-forget` skips the
// forget phase: even with a global retention policy of keep_daily=1 (which
// would otherwise collapse same-day snapshots), every snapshot from every
// backup invocation survives.
func TestRun_NoForget_SkipsForgetPhase(t *testing.T) {
	resetFixture(t)

	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")

	writeConfig(t, dir, configOpts{
		ResticRepoPath:      repoPath,
		ResticPassword:      "",
		IncludeNames:        nil,
		IncludeNamePatterns: nil,
		ExcludeNames:        nil,
		Retention:           config.RetentionConfig{KeepDaily: 1},
	})

	cfg := runConfig{Dir: dir, Stdin: nil, Env: nil}

	first := runConba(t, cfg, "run", "--no-forget")
	requireSuccess(t, first, "first conba run --no-forget")

	second := runConba(t, cfg, "run", "--no-forget")
	requireSuccess(t, second, "second conba run --no-forget")

	if strings.Contains(second.Stdout, "==> forget") {
		t.Fatalf("expected no `==> forget` header with --no-forget; stdout=%q", second.Stdout)
	}

	snaps := resticSnapshots(t, repoPath)

	// After two runs without forget, each container's snapshots from both
	// invocations survive. With keep_daily=1 these would collapse if forget
	// had run. mysql has 2 mounts × 2 runs = 4 snapshots; bind-excluded
	// has 1 mount × 2 runs = 2 snapshots.
	requireSnapshotCount(t, snaps, containerApp, 2)
	requireSnapshotCount(t, snaps, containerMySQL, 4)
	requireSnapshotCount(t, snaps, containerBindExcluded, 2)
}
