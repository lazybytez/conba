//go:build e2e

package e2e_test

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	tagPrefixContainer = "container="
	tagPrefixVolume    = "volume="
)

// snapshotsForContainer returns snapshots tagged "container=<containerName>".
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

// TestBackup_FreshRepo_DiscoversAllNonIgnoredTargets asserts that a fresh
// backup produces one snapshot per non-ignored container, zero for the
// label-disabled one, and that every snapshot carries both container= and
// volume= tags.
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

// allTags flattens every Tags entry across snaps for diagnostic messages.
func allTags(snaps []ResticSnapshot) []string {
	var all []string

	for _, snap := range snaps {
		all = append(all, snap.Tags...)
	}

	return all
}

// TestBackup_DryRun_NoSnapshots asserts that `conba backup --dry-run` writes
// no snapshots to the repository.
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

// TestBackup_RepeatedBackup_ProducesTwoSnapshotsPerTarget asserts that each
// non-ignored container has exactly two snapshots after two backup runs.
// Restic deduplicates data but still records one snapshot per invocation.
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

// TestBackup_DataMutationReflectedInDiff backs up twice with a file added
// between runs and asserts `restic diff` flags the new file with a `+` marker.
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

// requireSingleAppSnapshotID returns the ID of the sole app snapshot,
// failing the test if zero or more than one is present.
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

// requireNewAppSnapshotID returns the second app snapshot's ID, requiring
// exactly two to be present and one of them to match excludeID.
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

// containsAddedLine reports whether any line in diff contains both a `+`
// marker and filename. Avoids pinning restic's exact column layout.
func containsAddedLine(diff, filename string) bool {
	for line := range strings.SplitSeq(diff, "\n") {
		if strings.Contains(line, "+") && strings.Contains(line, filename) {
			return true
		}
	}

	return false
}
