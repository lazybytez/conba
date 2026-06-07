package cli_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/build"
	"github.com/lazybytez/conba/internal/cli"
	"github.com/lazybytez/conba/internal/report"
)

const cmdVersion = "version"

var errResticMissing = errors.New("restic not found")

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

	out := runVersionThroughRoot(t, "text")

	for _, want := range []string{"conba ", "go:", "restic:", "recommended"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q; got %q", want, out)
		}
	}
}

func TestVersionCommand_JSONOutput(t *testing.T) {
	t.Parallel()

	out := runVersionThroughRoot(t, "json")

	// The version event is the first NDJSON line; a mismatch warning, if any,
	// follows on its own line.
	firstLine, _, _ := strings.Cut(strings.TrimSpace(out), "\n")

	record := map[string]any{}

	err := json.Unmarshal([]byte(firstLine), &record)
	if err != nil {
		t.Fatalf("output is not valid JSON (%v): %q", err, firstLine)
	}

	if record["event"] != "version" {
		t.Errorf("event = %v, want version", record["event"])
	}

	if record["restic_recommended"] != build.RecommendedResticVersion {
		t.Errorf("restic_recommended = %v, want %q",
			record["restic_recommended"], build.RecommendedResticVersion)
	}

	if _, ok := record["restic_installed"]; !ok {
		t.Errorf("version event missing restic_installed field: %v", record)
	}
}

func TestEmitVersion_WarnsOnMismatch(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	cli.EmitVersion(report.New(report.ModeText, &buf, false), "0.18.1", "0.17.3", nil)

	out := buf.String()
	if !strings.Contains(out, "WARNING: installed restic 0.17.3 differs") {
		t.Errorf("expected mismatch warning, got %q", out)
	}
}

func TestEmitVersion_NoWarnOnPatchDiff(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	cli.EmitVersion(report.New(report.ModeText, &buf, false), "0.18.1", "0.18.5", nil)

	if strings.Contains(buf.String(), "WARNING") {
		t.Errorf("patch difference must not warn, got %q", buf.String())
	}
}

func TestEmitVersion_WarnsWhenResticNotFound(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	cli.EmitVersion(report.New(report.ModeText, &buf, false), "0.18.1", "", errResticMissing)

	out := buf.String()
	if !strings.Contains(out, "restic was not found") {
		t.Errorf("expected not-found warning, got %q", out)
	}

	if !strings.Contains(out, "not found") {
		t.Errorf("expected restic reported as not found, got %q", out)
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
