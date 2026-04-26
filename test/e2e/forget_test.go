//go:build e2e

package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/config"
)

// TestForget_GlobalRetention_AppliesToAllContainers verifies that with a
// global retention policy of keep_daily=1, two backup runs collapse to a
// single snapshot per (container, mount) pair. The two backups land on the
// same calendar day so restic keeps one snapshot per source per day.
func TestForget_GlobalRetention_AppliesToAllContainers(t *testing.T) {
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

	requireSuccess(t, runConba(t, cfg, "init"), "conba init")
	requireSuccess(t, runConba(t, cfg, "backup"), "first conba backup")
	requireSuccess(t, runConba(t, cfg, "backup"), "second conba backup")

	requireSuccess(t, runConba(t, cfg, "forget"), "conba forget")

	snaps := resticSnapshots(t, repoPath)

	requireSnapshotCount(t, snaps, containerApp, 1)
	requireSnapshotCount(t, snaps, containerMySQL, 2)
	requireSnapshotCount(t, snaps, containerBindExcluded, 1)
}

// TestForget_LabelOverride_TakesPrecedence verifies that with no global
// retention, only the labelled container (conba-e2e-app via compose label
// conba.retention: "1d") is forgotten; all other containers are skipped
// because neither label nor global supplies a policy.
func TestForget_LabelOverride_TakesPrecedence(t *testing.T) {
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

	requireSuccess(t, runConba(t, cfg, "init"), "conba init")
	requireSuccess(t, runConba(t, cfg, "backup"), "first conba backup")
	requireSuccess(t, runConba(t, cfg, "backup"), "second conba backup")

	forgetResult := runConba(t, cfg, "forget")
	requireSuccess(t, forgetResult, "conba forget")

	requireStdoutContains(t, forgetResult, "Skipped")

	snaps := resticSnapshots(t, repoPath)

	requireSnapshotCount(t, snaps, containerApp, 1)
	requireSnapshotCount(t, snaps, containerMySQL, 4)
	requireSnapshotCount(t, snaps, containerBindExcluded, 2)
}

// TestForget_DryRun_NoChange verifies that --dry-run does not remove any
// snapshots and that the output reports the dry-run mode.
func TestForget_DryRun_NoChange(t *testing.T) {
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

	requireSuccess(t, runConba(t, cfg, "init"), "conba init")
	requireSuccess(t, runConba(t, cfg, "backup"), "first conba backup")
	requireSuccess(t, runConba(t, cfg, "backup"), "second conba backup")

	beforeSnaps := resticSnapshots(t, repoPath)

	dryResult := runConba(t, cfg, "forget", "--dry-run")
	requireSuccess(t, dryResult, "conba forget --dry-run")

	requireStdoutContains(t, dryResult, "(dry-run)")
	requireStdoutContains(t, dryResult, "would succeed")

	afterSnaps := resticSnapshots(t, repoPath)

	if len(beforeSnaps) != len(afterSnaps) {
		t.Fatalf(
			"snapshot count changed across dry-run: before=%d after=%d",
			len(beforeSnaps), len(afterSnaps),
		)
	}
}

// TestForget_HostScoping_DoesNotTouchOtherHosts verifies that the default
// host-scoping behaviour leaves snapshots tagged for foreign hostnames
// untouched. Two foreign snapshots are seeded under hostname=other-host
// using the SAME container/volume tags as a real discovered target so the
// scoping check is meaningful: without --all-hosts, both foreign snapshots
// must survive even when the global retention policy would otherwise
// collapse them.
func TestForget_HostScoping_DoesNotTouchOtherHosts(t *testing.T) {
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

	requireSuccess(t, runConba(t, cfg, "init"), "conba init")
	requireSuccess(t, runConba(t, cfg, "backup"), "first conba backup")

	foreignSource := writeForeignSource(t, "host-scoping-data")
	foreignTags := []string{
		"container=" + containerApp,
		"volume=conba-e2e_conba-e2e-app-data",
		"hostname=other-host",
	}

	backupAsHost(t, repoPath, "other-host", foreignSource, foreignTags)
	backupAsHost(t, repoPath, "other-host", foreignSource, foreignTags)

	requireSuccess(t, runConba(t, cfg, "forget"), "conba forget")

	snaps := resticSnapshots(t, repoPath)

	foreignSurvivors := snapshotsByHostname(snaps, "other-host")
	if len(foreignSurvivors) != 2 {
		t.Fatalf(
			"expected both foreign-host snapshots to survive default forget "+
				"(host scoping must exclude them from policy); got %d (%v)",
			len(foreignSurvivors), describeSnapshots(snaps),
		)
	}
}

// TestForget_AllHosts_AffectsForeignSnapshots verifies that --all-hosts
// brings foreign-host snapshots into scope: two foreign snapshots tagged
// with the same container/volume as a real target (but on a different
// hostname) get reduced to one under keep_daily=1. Restic's default
// --group-by host,paths means each host still gets its own retention
// group; the test asserts the foreign group itself was reduced rather
// than left untouched as it would be without --all-hosts.
func TestForget_AllHosts_AffectsForeignSnapshots(t *testing.T) {
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

	requireSuccess(t, runConba(t, cfg, "init"), "conba init")
	requireSuccess(t, runConba(t, cfg, "backup"), "first conba backup")

	foreignSource := writeForeignSource(t, "all-hosts-data")
	foreignTags := []string{
		"container=" + containerApp,
		"volume=conba-e2e_conba-e2e-app-data",
		"hostname=other-host",
	}

	backupAsHost(t, repoPath, "other-host", foreignSource, foreignTags)
	backupAsHost(t, repoPath, "other-host", foreignSource, foreignTags)

	beforeSnaps := resticSnapshots(t, repoPath)

	beforeForeign := snapshotsByHostname(beforeSnaps, "other-host")
	if len(beforeForeign) != 2 {
		t.Fatalf(
			"setup precondition: expected 2 foreign snapshots before forget; got %d (%v)",
			len(beforeForeign), describeSnapshots(beforeForeign),
		)
	}

	requireSuccess(t, runConba(t, cfg, "forget", "--all-hosts"),
		"conba forget --all-hosts")

	afterSnaps := resticSnapshots(t, repoPath)

	afterForeign := snapshotsByHostname(afterSnaps, "other-host")
	if len(afterForeign) != 1 {
		t.Fatalf(
			"expected exactly 1 foreign snapshot after --all-hosts forget "+
				"with keep_daily=1; got %d (%v)",
			len(afterForeign), describeSnapshots(afterForeign),
		)
	}
}

// TestForget_Surgical_ContainerFlag verifies that surgical mode with
// --container scopes the forget to that container's snapshots only, leaving
// other containers untouched even though they have data eligible for
// retention under the global policy. The current host's tag is auto-injected
// (no --all-hosts) so this also exercises the default host-scoping path.
func TestForget_Surgical_ContainerFlag(t *testing.T) {
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

	requireSuccess(t, runConba(t, cfg, "init"), "conba init")
	requireSuccess(t, runConba(t, cfg, "backup"), "first conba backup")
	requireSuccess(t, runConba(t, cfg, "backup"), "second conba backup")

	requireSuccess(t,
		runConba(t, cfg, "forget", "--container", containerApp),
		"conba forget --container "+containerApp)

	snaps := resticSnapshots(t, repoPath)

	requireSnapshotCount(t, snaps, containerApp, 1)
	requireSnapshotCount(t, snaps, containerMySQL, 4)
	requireSnapshotCount(t, snaps, containerBindExcluded, 2)
}

// requireSnapshotCount fails the test when snaps does not contain exactly
// want snapshots tagged container=name.
func requireSnapshotCount(t *testing.T, snaps []ResticSnapshot, name string, want int) {
	t.Helper()

	got := len(snapshotsForContainer(snaps, name))
	if got != want {
		t.Fatalf(
			"expected %d snapshots tagged container=%s; got %d (all tags=%v)",
			want, name, got, allTags(snaps),
		)
	}
}

// snapshotsByHostname returns snapshots whose Hostname field equals host.
func snapshotsByHostname(snaps []ResticSnapshot, host string) []ResticSnapshot {
	var matches []ResticSnapshot

	for _, snap := range snaps {
		if snap.Hostname == host {
			matches = append(matches, snap)
		}
	}

	return matches
}

// describeSnapshots renders a compact human-readable snapshot summary for
// failure messages.
func describeSnapshots(snaps []ResticSnapshot) []string {
	out := make([]string, 0, len(snaps))

	for _, snap := range snaps {
		out = append(out, snap.ShortID+" host="+snap.Hostname+
			" tags="+strings.Join(snap.Tags, ","))
	}

	return out
}

// writeForeignSource creates a temporary source file whose contents include
// suffix so that successive backupAsHost calls on the same path produce
// distinct deduplication-resistant snapshots when needed.
func writeForeignSource(t *testing.T, suffix string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "foreign-data.txt")

	err := os.WriteFile(path, []byte("foreign-snapshot-"+suffix), 0o600)
	if err != nil {
		t.Fatalf("write foreign source: %v", err)
	}

	return path
}
