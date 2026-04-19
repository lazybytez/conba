//go:build e2e

package e2e_test

import (
	"path/filepath"
	"testing"
)

// TestInspect_IncludedAndExcludedSections runs `conba inspect` against the
// e2e fixture and asserts the output contains both section headers and
// their expected occupants. printSection (internal/cli/inspect.go:94)
// emits "=== <Title> ===" for each non-empty section; printIncluded and
// printExcluded (internal/cli/inspect.go:106-152) render the container
// name (and reason for exclusions). The exclusion reason text is produced
// by filter.evaluate (internal/filter/filter.go:62) when the
// conba.enabled=false label is detected on a target's container.
//
//nolint:paralleltest // Suite runs with -p 1; t.Parallel() is forbidden.
func TestInspect_IncludedAndExcludedSections(t *testing.T) {
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

	inspectResult := runConba(t, cfg, "inspect")
	requireExit(t, inspectResult, "conba inspect", 0)

	// Section headings — see printSection in internal/cli/inspect.go:94
	// (format string `"=== %s ===\n\n"` with titles "Included" and
	// "Excluded" passed from printResult, internal/cli/inspect.go:72,79).
	requireStdoutContains(t, inspectResult, "=== Included ===")
	requireStdoutContains(t, inspectResult, "=== Excluded ===")

	// Container names rendered by printIncluded (internal/cli/inspect.go:112)
	// and printExcluded (internal/cli/inspect.go:140). Both mysql and app
	// have no conba.enabled label so they pass the filter and appear under
	// Included; conba-e2e-ignored carries conba.enabled=false (see
	// test/e2e/compose.yaml:79) so it appears under Excluded.
	requireStdoutContains(t, inspectResult, containerMySQL)
	requireStdoutContains(t, inspectResult, containerApp)
	requireStdoutContains(t, inspectResult, containerIgnored)

	// Exclusion reason — see internal/filter/filter.go:62. printExcluded
	// emits the literal reason string after a "reason: " prefix
	// (internal/cli/inspect.go:140).
	requireStdoutContains(t, inspectResult, "excluded by conba.enabled=false label")
}
