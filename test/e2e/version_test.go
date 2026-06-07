//go:build e2e

package e2e_test

import "testing"

// TestVersion_PrintsBuildInfo asserts `conba version` exits 0 and prints the
// version line with the go and restic components. It needs no config file or
// repository: the command bypasses config loading via its own no-op
// PersistentPreRunE, so it runs from an empty working directory.
func TestVersion_PrintsBuildInfo(t *testing.T) {
	cfg := runConfig{Dir: t.TempDir(), Stdin: nil, Env: nil}

	result := runConba(t, cfg, "version", "--output", "text")
	requireSuccess(t, result, "conba version --output text")
	requireStdoutContains(t, result, "conba")
	requireStdoutContains(t, result, "go:")
	requireStdoutContains(t, result, "restic:")

	jsonResult := runConba(t, cfg, "version", "--output", "json")
	requireSuccess(t, jsonResult, "conba version --output json")
	requireEvent(t, jsonResult.Stdout, "version")
}
