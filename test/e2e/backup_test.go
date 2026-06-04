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
func TestBackup_FreshRepo_DiscoversAllNonIgnoredTargets(t *testing.T) {
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

	backupResult := runConba(t, cfg, "backup")
	requireSuccess(t, backupResult, "conba backup")

	snaps := resticSnapshots(t, repoPath)
	if len(snaps) == 0 {
		t.Fatalf("expected at least one snapshot after backup; got none")
	}

	requireAtLeastOneSnapshot(t, snaps, containerMySQL)
	requireAtLeastOneSnapshot(t, snaps, containerApp)
	requireAtLeastOneSnapshot(t, snaps, containerBindExcluded)
	requireNoSnapshots(t, snaps, containerIgnored)
	requireAllSnapshotsTagged(t, snaps)
}

// requireAtLeastOneSnapshot fails the test if no snapshot in snaps is tagged
// container=name.
func requireAtLeastOneSnapshot(t *testing.T, snaps []ResticSnapshot, name string) {
	t.Helper()

	matched := snapshotsForContainer(snaps, name)
	if len(matched) == 0 {
		t.Fatalf(
			"expected at least one snapshot tagged container=%s; tags=%v",
			name, allTags(snaps),
		)
	}
}

// requireNoSnapshots fails the test if any snapshot in snaps is tagged
// container=name.
func requireNoSnapshots(t *testing.T, snaps []ResticSnapshot, name string) {
	t.Helper()

	matched := snapshotsForContainer(snaps, name)
	if len(matched) != 0 {
		t.Fatalf(
			"expected zero snapshots tagged container=%s; got %d",
			name, len(matched),
		)
	}
}

// requireAllSnapshotsTagged fails the test if any snapshot is missing a
// container= or volume= tag.
func requireAllSnapshotsTagged(t *testing.T, snaps []ResticSnapshot) {
	t.Helper()

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
func TestBackup_DryRun_NoSnapshots(t *testing.T) {
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

	dryResult := runConba(t, cfg, "backup", "--dry-run")
	requireSuccess(t, dryResult, "conba backup --dry-run")

	snaps := resticSnapshots(t, repoPath)
	if len(snaps) != 0 {
		t.Fatalf("expected 0 snapshots after --dry-run; got %d", len(snaps))
	}
}

// TestBackup_RepeatedBackup_ProducesTwoSnapshotsPerTarget asserts that each
// non-ignored container has exactly two snapshots after two backup runs.
// Restic deduplicates data but still records one snapshot per invocation.
func TestBackup_RepeatedBackup_ProducesTwoSnapshotsPerTarget(t *testing.T) {
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

	firstBackup := runConba(t, cfg, "backup")
	requireSuccess(t, firstBackup, "first conba backup")

	secondBackup := runConba(t, cfg, "backup")
	requireSuccess(t, secondBackup, "second conba backup")

	snaps := resticSnapshots(t, repoPath)

	mysqlSnaps := snapshotsForContainer(snaps, containerMySQL)
	if len(mysqlSnaps) != 4 {
		t.Fatalf(
			"expected 4 snapshots tagged container=%s after two backups "+
				"(2 mounts × 2 runs); got %d (tags=%v)",
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

	bindExcludedSnaps := snapshotsForContainer(snaps, containerBindExcluded)
	if len(bindExcludedSnaps) != 2 {
		t.Fatalf(
			"expected 2 snapshots tagged container=%s after two backups "+
				"(1 named volume × 2 runs; bind is label-excluded); got %d (tags=%v)",
			containerBindExcluded, len(bindExcludedSnaps), allTags(snaps),
		)
	}

	ignoredSnaps := snapshotsForContainer(snaps, containerIgnored)
	if len(ignoredSnaps) != 0 {
		t.Fatalf("expected 0 snapshots tagged container=%s; got %d",
			containerIgnored, len(ignoredSnaps))
	}
}

// TestBackup_DataMutationReflectedInDiff backs up twice with a file added
// between runs and asserts `restic diff` flags the new file with a `+` marker.
func TestBackup_DataMutationReflectedInDiff(t *testing.T) {
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

	firstBackup := runConba(t, cfg, "backup")
	requireSuccess(t, firstBackup, "first conba backup")

	round1AppID := requireSingleAppSnapshotID(t, repoPath)

	composeExec(t, containerApp, nil, "sh", "-c", "echo fresh > /data/added.txt")

	secondBackup := runConba(t, cfg, "backup")
	requireSuccess(t, secondBackup, "second conba backup")

	round2AppID := requireNewAppSnapshotID(t, repoPath, round1AppID)

	diff := resticDiff(t, repoPath, round1AppID, round2AppID)

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

	snaps := resticSnapshots(t, repoPath)

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

	snaps := resticSnapshots(t, repoPath)

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

// TestBackup_BindMount_BackedUpByDefault asserts that, with no exclusion
// label set, a bind-mounted file on a container is backed up alongside its
// named volume. The mysql fixture has both: a named data volume and a bind
// mount of init.sql.
func TestBackup_BindMount_BackedUpByDefault(t *testing.T) {
	resetFixture(t)

	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")

	writeConfig(t, dir, configOpts{
		ResticRepoPath:           repoPath,
		ResticPassword:           "",
		IncludeNames:             nil,
		IncludeNamePatterns:      []string{"^conba-e2e-mysql$"},
		ExcludeNames:             nil,
		ResticEnvironment:        nil,
		PreBackupCommandsEnabled: false,
	})

	cfg := runConfig{Dir: dir, Stdin: nil, Env: nil}

	initResult := runConba(t, cfg, "init")
	requireSuccess(t, initResult, "conba init")

	backupResult := runConba(t, cfg, "backup")
	requireSuccess(t, backupResult, "conba backup")

	snaps := resticSnapshots(t, repoPath)

	mysqlSnaps := snapshotsForContainer(snaps, containerMySQL)
	if len(mysqlSnaps) != 2 {
		t.Fatalf(
			"expected 2 snapshots for %s (1 named volume + 1 bind mount); got %d (tags=%v)",
			containerMySQL, len(mysqlSnaps), allTags(snaps),
		)
	}

	volumeTags := volumeTagsOf(mysqlSnaps)
	if len(volumeTags) != 2 {
		t.Fatalf(
			"expected 2 distinct volume= tags across mysql snapshots; got %d (tags=%v)",
			len(volumeTags), volumeTags,
		)
	}

	if volumeTags[0] == volumeTags[1] {
		t.Fatalf(
			"expected the two mysql snapshots to have different volume= tags; both were %q",
			volumeTags[0],
		)
	}

	if !anyContains(volumeTags, "init.sql") {
		t.Fatalf(
			"expected at least one mysql snapshot's volume= tag to reference init.sql (the bind mount); volumeTags=%v",
			volumeTags,
		)
	}
}

// TestBackup_BindMount_ExcludedByDestinationLabel asserts that when a
// container carries the conba.exclude-mount-destinations label, the matching
// bind mount is excluded from backup while its named volume continues to be
// backed up.
func TestBackup_BindMount_ExcludedByDestinationLabel(t *testing.T) {
	resetFixture(t)

	dir := t.TempDir()
	repoPath := filepath.Join(dir, "repo")

	writeConfig(t, dir, configOpts{
		ResticRepoPath:           repoPath,
		ResticPassword:           "",
		IncludeNames:             nil,
		IncludeNamePatterns:      []string{"^conba-e2e-bind-excluded$"},
		ExcludeNames:             nil,
		ResticEnvironment:        nil,
		PreBackupCommandsEnabled: false,
	})

	cfg := runConfig{Dir: dir, Stdin: nil, Env: nil}

	initResult := runConba(t, cfg, "init")
	requireSuccess(t, initResult, "conba init")

	backupResult := runConba(t, cfg, "backup")
	requireSuccess(t, backupResult, "conba backup")

	snaps := resticSnapshots(t, repoPath)

	bindExcludedSnaps := snapshotsForContainer(snaps, containerBindExcluded)
	if len(bindExcludedSnaps) != 1 {
		t.Fatalf(
			"expected exactly 1 snapshot for %s (named volume only; "+
				"bind is label-excluded); got %d (tags=%v)",
			containerBindExcluded, len(bindExcludedSnaps), allTags(snaps),
		)
	}

	volumeTags := volumeTagsOf(bindExcludedSnaps)
	if len(volumeTags) != 1 {
		t.Fatalf(
			"expected exactly 1 volume= tag on the bind-excluded snapshot; got %d (tags=%v)",
			len(volumeTags), volumeTags,
		)
	}

	if strings.Contains(volumeTags[0], "init.sql") {
		t.Fatalf(
			"expected the bind-excluded container's surviving snapshot to "+
				"NOT reference init.sql (bind should be excluded); volumeTag=%q",
			volumeTags[0],
		)
	}
}

// volumeTagsOf returns the value of the volume= tag on each snapshot, in
// snapshot order. Snapshots without a volume= tag are skipped.
func volumeTagsOf(snaps []ResticSnapshot) []string {
	var tags []string

	for _, snap := range snaps {
		for _, tag := range snap.Tags {
			value, ok := strings.CutPrefix(tag, tagPrefixVolume)
			if !ok {
				continue
			}

			tags = append(tags, value)

			break
		}
	}

	return tags
}

// anyContains reports whether any element of values contains substr.
func anyContains(values []string, substr string) bool {
	for _, v := range values {
		if strings.Contains(v, substr) {
			return true
		}
	}

	return false
}
