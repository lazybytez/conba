//go:build e2e

package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// Volume names below match the docker compose project naming. The compose
// project is named "conba-e2e" (test/e2e/compose.yaml:11), so each named
// volume is prefixed with the project name on disk and in container
// inspect output. backup.BuildTags (internal/backup/tags.go:14) tags every
// snapshot with "volume=" + target.Mount.Name, where Mount.Name is the
// resolved docker volume name from internal/runtime/docker/docker.go:93,
// hence the project-prefixed strings here.
const (
	volumeMySQL = "conba-e2e_conba-e2e-mysql-data"
	volumeApp   = "conba-e2e_conba-e2e-app-data"
)

// TestSnapshots_FilterByContainer runs `conba init`, `conba backup` and
// then `conba snapshots --container conba-e2e-app`. The snapshots command
// passes the flag through to restic as a "container=<name>" tag filter
// (see runSnapshots / buildFilterTags in internal/cli/snapshots.go:73,151)
// so only app-tagged snapshots come back. The tabular output emitted by
// printSnapshots (internal/cli/snapshots.go:110-147) renders the volume
// column for each snapshot, which we use to assert the app's volume is
// present and the mysql container/volume names are absent.
//
//nolint:paralleltest // Suite runs with -p 1; t.Parallel() is forbidden.
func TestSnapshots_FilterByContainer(t *testing.T) {
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

	snapsResult := runConba(t, cfg, "snapshots", "--container", containerApp)
	requireExit(t, snapsResult, "conba snapshots --container conba-e2e-app", 0)

	// printSnapshots renders the app's volume name in the Volume column
	// (internal/cli/snapshots.go:119-125 via extractTag with tagPrefixVolume).
	requireStdoutContains(t, snapsResult, volumeApp)

	// The mysql container name appears in the Container column for any
	// mysql-tagged snapshot (internal/cli/snapshots.go:122) — its absence
	// proves the --container filter is being honoured.
	if strings.Contains(snapsResult.Stdout, containerMySQL) {
		t.Fatalf(
			"snapshots --container %s stdout unexpectedly mentions %q; stdout=%q",
			containerApp, containerMySQL, snapsResult.Stdout,
		)
	}

	// Likewise the mysql volume name must not appear; same column source as
	// volumeApp above (internal/cli/snapshots.go:123).
	if strings.Contains(snapsResult.Stdout, volumeMySQL) {
		t.Fatalf(
			"snapshots --container %s stdout unexpectedly mentions %q; stdout=%q",
			containerApp, volumeMySQL, snapsResult.Stdout,
		)
	}
}

// TestSnapshots_FilterByVolume runs `conba init`, `conba backup` and then
// `conba snapshots --volume <mysql-volume>`. The --volume flag is
// converted into a "volume=<name>" tag filter by buildFilterTags
// (internal/cli/snapshots.go:151-167) and forwarded to restic via
// snapshotFilters.tags (internal/cli/snapshots.go:35-37). The remaining
// snapshots all carry the mysql volume tag, which printSnapshots emits in
// the Volume column (internal/cli/snapshots.go:123).
//
//nolint:paralleltest // Suite runs with -p 1; t.Parallel() is forbidden.
func TestSnapshots_FilterByVolume(t *testing.T) {
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

	snapsResult := runConba(t, cfg, "snapshots", "--volume", volumeMySQL)
	requireExit(t, snapsResult, "conba snapshots --volume "+volumeMySQL, 0)

	// printSnapshots renders the mysql volume name in the Volume column
	// (internal/cli/snapshots.go:123 via extractTag with tagPrefixVolume).
	requireStdoutContains(t, snapsResult, volumeMySQL)

	// The app volume name must not appear in the filtered listing — same
	// column source (internal/cli/snapshots.go:123).
	if strings.Contains(snapsResult.Stdout, volumeApp) {
		t.Fatalf(
			"snapshots --volume %s stdout unexpectedly mentions %q; stdout=%q",
			volumeMySQL, volumeApp, snapsResult.Stdout,
		)
	}
}
