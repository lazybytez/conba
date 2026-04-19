//go:build e2e

package e2e_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Tag format under test is produced by backup.BuildTags in
// internal/backup/tags.go:11-16 and joined comma-separated by appendTags
// in internal/restic/args.go:59-65. Every snapshot carries exactly:
//
//	"container=<container-name>", "volume=<mount-name>", "hostname=<hostname>"
//
// Helpers below filter snapshots on the canonical "container=" tag.

const (
	tagPrefixContainer = "container="
	tagPrefixVolume    = "volume="
)

// snapshotsForContainer returns the subset of snaps whose Tags contain the
// tag "container=<containerName>" produced by backup.BuildTags
// (internal/backup/tags.go:13).
func snapshotsForContainer(snaps []ResticSnapshot, containerName string) []ResticSnapshot {
	want := tagPrefixContainer + containerName

	var matches []ResticSnapshot

	for _, snap := range snaps {
		if slices.Contains(snap.Tags, want) {
			matches = append(matches, snap)
		}
	}

	return matches
}

// hasTagWithPrefix reports whether any tag in tags starts with prefix.
func hasTagWithPrefix(tags []string, prefix string) bool {
	for _, tag := range tags {
		if strings.HasPrefix(tag, prefix) {
			return true
		}
	}

	return false
}

// TestBackup_FreshRepo_DiscoversAllNonIgnoredTargets runs `conba init` then
// `conba backup` against a freshly-created repo and asserts:
//   - mysql and app containers each produce at least one snapshot
//   - the conba-e2e-ignored container (labelled conba.enabled=false) is
//     filtered out by internal/filter/filter.go and produces zero
//   - every snapshot carries both a container= and a volume= tag, matching
//     the format emitted by internal/backup/tags.go.
//
//nolint:paralleltest // Suite runs with -p 1; t.Parallel() is forbidden.
func TestBackup_FreshRepo_DiscoversAllNonIgnoredTargets(t *testing.T) {
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

	backupResult := runConba(t, cfg, "backup")
	requireExit(t, backupResult, "conba backup", 0)

	snaps := resticSnapshots(t, repoPath, defaultResticPassword)
	if len(snaps) == 0 {
		t.Fatalf("expected at least one snapshot after backup; got none")
	}

	mysqlSnaps := snapshotsForContainer(snaps, containerMySQL)
	if len(mysqlSnaps) == 0 {
		t.Fatalf(
			"expected at least one snapshot tagged container=%s; tags=%v",
			containerMySQL, allTags(snaps),
		)
	}

	appSnaps := snapshotsForContainer(snaps, containerApp)
	if len(appSnaps) == 0 {
		t.Fatalf(
			"expected at least one snapshot tagged container=%s; tags=%v",
			containerApp, allTags(snaps),
		)
	}

	ignoredSnaps := snapshotsForContainer(snaps, containerIgnored)
	if len(ignoredSnaps) != 0 {
		t.Fatalf(
			"expected zero snapshots tagged container=%s (filtered by conba.enabled=false); got %d",
			containerIgnored, len(ignoredSnaps),
		)
	}

	for _, snap := range snaps {
		if !hasTagWithPrefix(snap.Tags, tagPrefixContainer) {
			t.Fatalf("snapshot %s missing container= tag; tags=%v", snap.ShortID, snap.Tags)
		}

		if !hasTagWithPrefix(snap.Tags, tagPrefixVolume) {
			t.Fatalf("snapshot %s missing volume= tag; tags=%v", snap.ShortID, snap.Tags)
		}
	}
}

// allTags returns a flat slice of every Tags entry across snaps, used only
// for diagnostic messages on assertion failure.
func allTags(snaps []ResticSnapshot) []string {
	var all []string

	for _, snap := range snaps {
		all = append(all, snap.Tags...)
	}

	return all
}

// TestBackup_DryRun_NoSnapshots verifies that `conba backup --dry-run` does
// not write any snapshots to the restic repository. printDryRun in
// internal/cli/backup.go:108-150 prints the planned targets and returns
// without invoking restic backup.
//
//nolint:paralleltest // Suite runs with -p 1; t.Parallel() is forbidden.
func TestBackup_DryRun_NoSnapshots(t *testing.T) {
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

	dryResult := runConba(t, cfg, "backup", "--dry-run")
	requireExit(t, dryResult, "conba backup --dry-run", 0)

	snaps := resticSnapshots(t, repoPath, defaultResticPassword)
	if len(snaps) != 0 {
		t.Fatalf("expected 0 snapshots after --dry-run; got %d", len(snaps))
	}
}

// TestBackup_RepeatedBackup_ProducesTwoSnapshotsPerTarget runs backup twice
// in a row and asserts that each non-ignored container has exactly two
// snapshots tagged with its container= tag. Restic's content-addressed
// store deduplicates data but still creates a new snapshot metadata entry
// per invocation, so snapshot count grows monotonically with backup runs.
//
//nolint:paralleltest // Suite runs with -p 1; t.Parallel() is forbidden.
func TestBackup_RepeatedBackup_ProducesTwoSnapshotsPerTarget(t *testing.T) {
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

	firstBackup := runConba(t, cfg, "backup")
	requireExit(t, firstBackup, "first conba backup", 0)

	secondBackup := runConba(t, cfg, "backup")
	requireExit(t, secondBackup, "second conba backup", 0)

	snaps := resticSnapshots(t, repoPath, defaultResticPassword)

	mysqlSnaps := snapshotsForContainer(snaps, containerMySQL)
	if len(mysqlSnaps) != 2 {
		t.Fatalf(
			"expected 2 snapshots tagged container=%s after two backups; got %d (tags=%v)",
			containerMySQL, len(mysqlSnaps), allTags(snaps),
		)
	}

	appSnaps := snapshotsForContainer(snaps, containerApp)
	if len(appSnaps) != 2 {
		t.Fatalf(
			"expected 2 snapshots tagged container=%s after two backups; got %d (tags=%v)",
			containerApp, len(appSnaps), allTags(snaps),
		)
	}

	ignoredSnaps := snapshotsForContainer(snaps, containerIgnored)
	if len(ignoredSnaps) != 0 {
		t.Fatalf(
			"expected 0 snapshots tagged container=%s; got %d",
			containerIgnored, len(ignoredSnaps),
		)
	}
}

// TestBackup_DataMutationReflectedInDiff backs up the app volume, mutates
// the volume content via `docker exec`, backs up again, and asserts that
// `restic diff round1 round2` mentions the new file with a `+` marker —
// restic's diff format emits `+    /path/to/file` for added entries.
//
// The two snapshot IDs are extracted from resticSnapshots filtered to the
// app container and sorted by restic's natural JSON ordering (older first).
//
//nolint:paralleltest // Suite runs with -p 1; t.Parallel() is forbidden.
func TestBackup_DataMutationReflectedInDiff(t *testing.T) {
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

	firstBackup := runConba(t, cfg, "backup")
	requireExit(t, firstBackup, "first conba backup", 0)

	round1AppID := requireSingleAppSnapshotID(t, repoPath)

	dockerExec(t, containerApp, "sh", "-c", "echo fresh > /data/added.txt")

	secondBackup := runConba(t, cfg, "backup")
	requireExit(t, secondBackup, "second conba backup", 0)

	round2AppID := requireNewAppSnapshotID(t, repoPath, round1AppID)

	diff := resticDiff(t, repoPath, defaultResticPassword, round1AppID, round2AppID)

	if !strings.Contains(diff, "added.txt") {
		t.Fatalf("restic diff does not mention added.txt:\n%s", diff)
	}

	if !containsAddedLine(diff, "added.txt") {
		t.Fatalf("restic diff has no `+` marker next to added.txt:\n%s", diff)
	}
}

// requireSingleAppSnapshotID loads snapshots for the app container and
// returns the ID of the only entry, failing the test otherwise.
func requireSingleAppSnapshotID(t *testing.T, repoPath string) string {
	t.Helper()

	snaps := resticSnapshots(t, repoPath, defaultResticPassword)

	appSnaps := snapshotsForContainer(snaps, containerApp)
	if len(appSnaps) != 1 {
		t.Fatalf(
			"expected 1 app snapshot after first backup; got %d (tags=%v)",
			len(appSnaps), allTags(snaps),
		)
	}

	return appSnaps[0].ID
}

// requireNewAppSnapshotID expects exactly two app-container snapshots and
// returns the ID that differs from excludeID.
func requireNewAppSnapshotID(t *testing.T, repoPath, excludeID string) string {
	t.Helper()

	snaps := resticSnapshots(t, repoPath, defaultResticPassword)

	appSnaps := snapshotsForContainer(snaps, containerApp)
	if len(appSnaps) != 2 {
		t.Fatalf(
			"expected 2 app snapshots after second backup; got %d (tags=%v)",
			len(appSnaps), allTags(snaps),
		)
	}

	for _, snap := range appSnaps {
		if snap.ID != excludeID {
			return snap.ID
		}
	}

	t.Fatalf("could not find new app snapshot (exclude=%s, all=%v)", excludeID, appSnaps)

	return ""
}

// containsAddedLine reports whether diff contains a line that has both a
// `+` marker and the given filename. restic diff uses lines like
// "+    /data/added.txt"; we assert the marker and name appear on the
// same line without pinning the exact column.
func containsAddedLine(diff, filename string) bool {
	for line := range strings.SplitSeq(diff, "\n") {
		if strings.Contains(line, "+") && strings.Contains(line, filename) {
			return true
		}
	}

	return false
}
