package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/build"
	"github.com/lazybytez/conba/internal/cli"
	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/report"
	"github.com/lazybytez/conba/internal/restic"
	"github.com/spf13/cobra"
)

const (
	cmdVersion         = "version"
	probeBinary        = "/opt/restic/restic"
	recommendedVersion = "0.18.1"
)

var (
	errExitStatus = errors.New("exit status 3")
	errNoisyCause = errors.New("boom\nrestic: fake")

	errResticMissing    = fmt.Errorf("running %q version: %w", probeBinary, exec.ErrNotFound)
	errResticUnreadable = fmt.Errorf("running probe: %w", restic.ErrResticVersionParse)
	errResticUnrunnable = fmt.Errorf("running %q version: %w", probeBinary, fs.ErrPermission)
)

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

	wantPrefix := fmt.Sprintf(
		"conba %s (go: %s)\nrestic: ",
		build.ComputeVersionString(), build.GoVersion(),
	)
	if !strings.HasPrefix(out, wantPrefix) {
		t.Errorf("text output = %q, want prefix %q", out, wantPrefix)
	}

	wantRecommended := fmt.Sprintf("(recommended %s)", build.RecommendedResticVersion)
	if !strings.Contains(out, wantRecommended) {
		t.Errorf("text output = %q, want it to contain %q", out, wantRecommended)
	}
}

func TestVersionCommand_JSONOutput(t *testing.T) {
	t.Parallel()

	out := runVersionThroughRoot(t, "json")

	// The version event is the first NDJSON line. A mismatch warning, if any,
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

	if record["restic_binary"] != config.DefaultResticBinary {
		t.Errorf("restic_binary = %v, want %q",
			record["restic_binary"], config.DefaultResticBinary)
	}
}

func TestResolveVersionProbeBinary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		write   bool
		content string
		want    string
	}{
		{
			name:    "configured binary",
			write:   true,
			content: "restic:\n  binary: /opt/restic/restic\n",
			want:    "/opt/restic/restic",
		},
		{
			name:    "explicit path to a missing file",
			write:   false,
			content: "",
			want:    config.DefaultResticBinary,
		},
		{
			name:    "malformed config",
			write:   true,
			content: "restic: [unterminated\n",
			want:    config.DefaultResticBinary,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfgFile := filepath.Join(t.TempDir(), "conba.yaml")
			if test.write {
				writeVersionConfig(t, cfgFile, test.content)
			}

			got := cli.ResolveVersionProbeBinary(commandWithConfigFlag(cfgFile))
			if got != test.want {
				t.Errorf("ResolveVersionProbeBinary() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestEmitVersion_TextMessageNamesProbedBinary(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	cli.EmitVersion(
		report.New(report.ModeText, &buf, false),
		probeBinary, "0.18.1", "0.18.1", nil,
	)

	want := `restic: 0.18.1 at "/opt/restic/restic" (recommended 0.18.1)`
	if !strings.Contains(buf.String(), want) {
		t.Errorf("version message must name the probed binary, got %q", buf.String())
	}
}

// TestEmitVersion_EscapesUntrustedBinary asserts the configured binary path,
// which the config file controls, cannot carry escapes into the terminal.
func TestEmitVersion_EscapesUntrustedBinary(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	cli.EmitVersion(
		report.New(report.ModeText, &buf, false),
		"/opt/\x1b[31mrestic\nrestic: fake", "0.18.1", "0.18.1", nil,
	)

	out := buf.String()
	if strings.Contains(out, "\x1b") {
		t.Errorf("version message must escape control sequences, got %q", out)
	}

	if strings.Contains(out, "\nrestic: fake") {
		t.Errorf("version message must escape newlines, got %q", out)
	}
}

func TestEmitVersion_NotFoundWarningNamesProbedBinary(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	cli.EmitVersion(
		report.New(report.ModeText, &buf, false),
		probeBinary, "0.18.1", "", errResticMissing,
	)

	if !strings.Contains(buf.String(), `"/opt/restic/restic"`) {
		t.Errorf("not-found warning must name the probed binary, got %q", buf.String())
	}
}

func TestEmitVersion_NotFoundWarningCarriesBinaryField(t *testing.T) {
	t.Parallel()

	events := emitVersionEvents(t, errResticMissing)

	warning, ok := events["restic.not_found"]
	if !ok {
		t.Fatalf("expected a restic.not_found event, got %v", events)
	}

	if warning["binary"] != probeBinary {
		t.Errorf("restic.not_found binary = %v, want %q", warning["binary"], probeBinary)
	}
}

func TestEmitVersion_InstalledFieldEmptyWhenProbeFails(t *testing.T) {
	t.Parallel()

	events := emitVersionEvents(t, errResticMissing)

	if events[cmdVersion]["restic_installed"] != "" {
		t.Errorf("restic_installed = %v, want an empty value",
			events[cmdVersion]["restic_installed"])
	}
}

func TestEmitVersion_WarnsWhenVersionOutputUnreadable(t *testing.T) {
	t.Parallel()

	events := emitVersionEvents(t, errResticUnreadable)

	warning, ok := events["restic.version_unreadable"]
	if !ok {
		t.Fatalf("expected a restic.version_unreadable event, got %v", events)
	}

	if warning["binary"] != probeBinary {
		t.Errorf("restic.version_unreadable binary = %v, want %q",
			warning["binary"], probeBinary)
	}

	if _, found := events["restic.not_found"]; found {
		t.Error("an unreadable version output must not be reported as a missing restic")
	}
}

func TestEmitVersion_UnreadableTextMessageSaysResticRan(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	cli.EmitVersion(
		report.New(report.ModeText, &buf, false),
		probeBinary, "0.18.1", "", errResticUnreadable,
	)

	out := buf.String()
	if !strings.Contains(out, "version output could not be read") {
		t.Errorf("expected an unreadable-version warning, got %q", out)
	}

	if !strings.Contains(out, `restic: unreadable at "`+probeBinary+`"`) {
		t.Errorf("expected the version line to report restic as unreadable, got %q", out)
	}

	if strings.Contains(out, "not found") {
		t.Errorf("a binary that ran must not be reported as not found, got %q", out)
	}
}

// TestEmitVersion_ProbeFailureEventName pins which warning a probe failure
// produces: only a binary that is genuinely absent may be reported as missing.
func TestEmitVersion_ProbeFailureEventName(t *testing.T) {
	t.Parallel()

	allNames := []string{"restic.not_found", "restic.version_unreadable", "restic.probe_failed"}

	tests := []struct {
		name      string
		detectErr error
		want      string
	}{
		{"bare name not on PATH", errResticMissing, "restic.not_found"},
		{"path does not exist", fmt.Errorf("probing: %w", fs.ErrNotExist), "restic.not_found"},
		{"output did not parse", errResticUnreadable, "restic.version_unreadable"},
		{"permission denied", errResticUnrunnable, "restic.probe_failed"},
		{"non-zero exit", fmt.Errorf("probing: %w", errExitStatus), "restic.probe_failed"},
		{
			"probe timed out",
			fmt.Errorf("probing: %w", context.DeadlineExceeded),
			"restic.probe_failed",
		},
		{
			"lingering descendant",
			fmt.Errorf("probing: %w", exec.ErrWaitDelay),
			"restic.probe_failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			events := emitVersionEvents(t, test.detectErr)

			warning, ok := events[test.want]
			if !ok {
				t.Fatalf("expected a %s event, got %v", test.want, events)
			}

			if warning["binary"] != probeBinary {
				t.Errorf("%s binary = %v, want %q", test.want, warning["binary"], probeBinary)
			}

			for _, name := range allNames {
				if name == test.want {
					continue
				}

				if _, found := events[name]; found {
					t.Errorf("probe failure %v must not emit %s", test.detectErr, name)
				}
			}
		})
	}
}

func TestEmitVersion_ProbeFailedTextMessageSaysResticExists(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	cli.EmitVersion(
		report.New(report.ModeText, &buf, false),
		probeBinary, "0.18.1", "", errResticUnrunnable,
	)

	out := buf.String()
	if !strings.Contains(out, "could not be run") {
		t.Errorf("expected a probe-failure warning, got %q", out)
	}

	if !strings.Contains(out, "permission denied") {
		t.Errorf("probe-failure warning must carry the underlying reason, got %q", out)
	}

	if !strings.Contains(out, `restic: not runnable at "`+probeBinary+`"`) {
		t.Errorf("expected the version line to report restic as not runnable, got %q", out)
	}

	if strings.Contains(out, "not found") {
		t.Errorf("a binary that exists must not be reported as not found, got %q", out)
	}
}

// TestEmitVersion_ProbeFailedWarningIsOneLine keeps the reason from breaking
// the warning across lines, where it could impersonate another event.
func TestEmitVersion_ProbeFailedWarningIsOneLine(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	cli.EmitVersion(
		report.New(report.ModeText, &buf, false),
		probeBinary, "0.18.1", "",
		fmt.Errorf("running probe: %w", errNoisyCause),
	)

	_, warning, found := strings.Cut(buf.String(), "WARNING:")
	if !found {
		t.Fatalf("expected a probe-failure warning, got %q", buf.String())
	}

	if strings.Contains(strings.TrimSuffix(warning, "\n"), "\n") {
		t.Errorf("probe-failure warning must stay on one line, got %q", warning)
	}
}

func TestEmitVersion_WarnsOnMismatch(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	cli.EmitVersion(
		report.New(report.ModeText, &buf, false),
		config.DefaultResticBinary, "0.18.1", "0.17.3", nil,
	)

	out := buf.String()
	if !strings.Contains(out, "WARNING: installed restic 0.17.3 differs") {
		t.Errorf("expected mismatch warning, got %q", out)
	}
}

func TestEmitVersion_NoWarnOnPatchDiff(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	cli.EmitVersion(
		report.New(report.ModeText, &buf, false),
		config.DefaultResticBinary, "0.18.1", "0.18.5", nil,
	)

	if strings.Contains(buf.String(), "WARNING") {
		t.Errorf("patch difference must not warn, got %q", buf.String())
	}
}

func TestEmitVersion_WarnsWhenResticNotFound(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	cli.EmitVersion(
		report.New(report.ModeText, &buf, false),
		config.DefaultResticBinary, "0.18.1", "", errResticMissing,
	)

	out := buf.String()
	if !strings.Contains(out, "restic was not found") {
		t.Errorf("expected not-found warning, got %q", out)
	}

	if !strings.Contains(out, `restic: not found at "`+config.DefaultResticBinary+`"`) {
		t.Errorf("expected the version line to report restic as not found, got %q", out)
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

func writeVersionConfig(t *testing.T, path, content string) {
	t.Helper()

	err := os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatalf("WriteFile() returned error: %v", err)
	}
}

// emitVersionEvents renders EmitVersion for a failed probe as JSON and returns
// the emitted records keyed by event name.
func emitVersionEvents(t *testing.T, detectErr error) map[string]map[string]any {
	t.Helper()

	var buf bytes.Buffer

	cli.EmitVersion(
		report.New(report.ModeJSON, &buf, false),
		probeBinary, recommendedVersion, "", detectErr,
	)

	events := map[string]map[string]any{}
	decoder := json.NewDecoder(&buf)

	for decoder.More() {
		record := map[string]any{}

		err := decoder.Decode(&record)
		if err != nil {
			t.Fatalf("decoding event stream: %v", err)
		}

		name, _ := record["event"].(string)
		events[name] = record
	}

	return events
}

func commandWithConfigFlag(cfgFile string) *cobra.Command {
	cmd := &cobra.Command{Use: cmdVersion}
	cmd.Flags().String("config", cfgFile, "path to config file")

	return cmd
}
