//go:build e2e

package e2e_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// missingResticBinary is a path that cannot exist, so the probe always
// fails and the not-found warning is emitted.
const missingResticBinary = "/nonexistent/restic"

// TestVersion_PrintsBuildInfo asserts `conba version` exits 0 and prints the
// version line with the go and restic components. It runs from an empty
// working directory, so it also covers the case where no config file exists.
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

// TestVersion_ProbesConfiguredResticBinary asserts `conba version` probes the
// binary configured in conba.yaml and still exits 0 when it is absent, naming
// the configured path in both render modes.
func TestVersion_ProbesConfiguredResticBinary(t *testing.T) {
	dir := t.TempDir()
	writeResticBinaryConfig(t, dir, missingResticBinary)

	cfg := runConfig{Dir: dir, Stdin: nil, Env: nil}

	result := runConba(t, cfg, "version", "--output", "text")
	requireSuccess(t, result, "conba version --output text")
	requireStdoutContains(t, result, fmt.Sprintf(
		"restic: not found at %q", missingResticBinary,
	))
	requireStdoutContains(t, result, fmt.Sprintf(
		"WARNING: restic was not found (probed %q)", missingResticBinary,
	))

	jsonResult := runConba(t, cfg, "version", "--output", "json")
	requireSuccess(t, jsonResult, "conba version --output json")
	requireEvent(t, jsonResult.Stdout, "restic.not_found")

	event := requireEvent(t, jsonResult.Stdout, "version")
	if event["restic_binary"] != missingResticBinary {
		t.Errorf(
			"version event restic_binary = %v, want %q",
			event["restic_binary"], missingResticBinary,
		)
	}
}

// writeResticBinaryConfig renders a conba.yaml into dir that sets only
// restic.binary, the single field the version command reads.
func writeResticBinaryConfig(t *testing.T, dir, binary string) {
	t.Helper()

	path := filepath.Join(dir, conbaConfigFilename)

	err := os.WriteFile(
		path,
		fmt.Appendf(nil, "restic:\n  binary: %q\n", binary),
		0o600,
	)
	if err != nil {
		t.Fatalf("write config to %q: %v", path, err)
	}

	verifyConfigLoads(t, path)
}
