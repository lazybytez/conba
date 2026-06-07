package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/build"
	"github.com/lazybytez/conba/internal/cli"
)

const cmdVersion = "version"

func TestNewVersionCommand_Use(t *testing.T) {
	t.Parallel()

	cmd := cli.NewVersionCommand()
	if cmd.Use != cmdVersion {
		t.Errorf("Use = %q, want %q", cmd.Use, "version")
	}
}

func TestNewVersionCommand_Short(t *testing.T) {
	t.Parallel()

	cmd := cli.NewVersionCommand()
	if cmd.Short == "" {
		t.Error("Short description must not be empty")
	}
}

func TestNewVersionCommand_PersistentPreRunE_SkipsConfigLoading(t *testing.T) {
	t.Parallel()

	cmd := cli.NewVersionCommand()
	if cmd.PersistentPreRunE == nil {
		t.Fatal("PersistentPreRunE must be set to skip config loading")
	}

	err := cmd.PersistentPreRunE(cmd, nil)
	if err != nil {
		t.Errorf("PersistentPreRunE() returned error: %v", err)
	}
}

func TestVersionCommand_TextOutput(t *testing.T) {
	t.Parallel()

	buf := runVersionThroughRoot(t, "text")

	want := "conba " + build.ComputeVersionString() +
		" (go: " + build.GoVersion() +
		", restic: " + build.ResticVersion + ")\n"

	if buf != want {
		t.Errorf("output = %q, want %q", buf, want)
	}
}

func TestVersionCommand_JSONOutput(t *testing.T) {
	t.Parallel()

	out := runVersionThroughRoot(t, "json")

	record := map[string]any{}

	err := json.Unmarshal([]byte(strings.TrimSpace(out)), &record)
	if err != nil {
		t.Fatalf("output is not valid JSON (%v): %q", err, out)
	}

	if record["event"] != "version" {
		t.Errorf("event = %v, want version", record["event"])
	}

	if record["restic"] != build.ResticVersion {
		t.Errorf("restic = %v, want %q", record["restic"], build.ResticVersion)
	}
}

func runVersionThroughRoot(t *testing.T, format string) string {
	t.Helper()

	root := cli.NewRootCommand()

	var buf bytes.Buffer

	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{cmdVersion, "--output", format})

	err := root.Execute()
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	return buf.String()
}
