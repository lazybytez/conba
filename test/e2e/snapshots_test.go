//go:build e2e

package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// Docker prefixes named volumes with the compose project ("conba-e2e"),
// so on-disk and inspect output carry the doubled prefix seen here.
const (
	volumeMySQL = "conba-e2e_conba-e2e-mysql-data"
	volumeApp   = "conba-e2e_conba-e2e-app-data"
)

// TestSnapshots_FilterByContainer asserts that `snapshots --container <app>`
// lists the app volume and omits any mysql container or volume name.
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

	requireStdoutContains(t, snapsResult, volumeApp)

	if strings.Contains(snapsResult.Stdout, containerMySQL) {
		t.Fatalf(
			"snapshots --container %s stdout unexpectedly mentions %q; stdout=%q",
			containerApp, containerMySQL, snapsResult.Stdout,
		)
	}

	if strings.Contains(snapsResult.Stdout, volumeMySQL) {
		t.Fatalf(
			"snapshots --container %s stdout unexpectedly mentions %q; stdout=%q",
			containerApp, volumeMySQL, snapsResult.Stdout,
		)
	}
}

// TestSnapshots_FilterByVolume asserts that `snapshots --volume <mysql>`
// lists only the mysql volume and omits the app volume.
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

	requireStdoutContains(t, snapsResult, volumeMySQL)

	if strings.Contains(snapsResult.Stdout, volumeApp) {
		t.Fatalf(
			"snapshots --volume %s stdout unexpectedly mentions %q; stdout=%q",
			volumeMySQL, volumeApp, snapsResult.Stdout,
		)
	}
}
