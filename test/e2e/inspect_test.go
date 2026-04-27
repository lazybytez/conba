//go:build e2e

package e2e_test

import (
	"path/filepath"
	"testing"
)

// TestInspect_IncludedAndExcludedSections asserts that `conba inspect` lists
// mysql and app under Included and the label-disabled container under Excluded.
func TestInspect_IncludedAndExcludedSections(t *testing.T) {
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

	inspectResult := runConba(t, cfg, "inspect")
	requireSuccess(t, inspectResult, "conba inspect")

	requireStdoutContains(t, inspectResult, "=== Included ===")
	requireStdoutContains(t, inspectResult, "=== Excluded ===")

	// conba-e2e-ignored carries conba.enabled=false in the fixture compose
	// file, so it must land under Excluded rather than Included.
	requireStdoutContains(t, inspectResult, containerMySQL)
	requireStdoutContains(t, inspectResult, containerApp)
	requireStdoutContains(t, inspectResult, containerIgnored)

	requireStdoutContains(t, inspectResult, "excluded by conba.enabled=false label")
}
