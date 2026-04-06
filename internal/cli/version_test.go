package cli_test

import (
	"bytes"
	"testing"

	"github.com/lazybytez/conba/internal/build"
	"github.com/lazybytez/conba/internal/cli"
)

func TestNewVersionCommand_Use(t *testing.T) {
	t.Parallel()

	cmd := cli.NewVersionCommand()
	if cmd.Use != "version" {
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

func TestNewVersionCommand_OutputFormat(t *testing.T) {
	t.Parallel()

	cmd := cli.NewVersionCommand()

	var buf bytes.Buffer

	cmd.SetOut(&buf)

	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("RunE() returned error: %v", err)
	}

	got := buf.String()
	want := "conba " + build.ComputeVersionString() +
		" (go: " + build.GoVersion() +
		", restic: " + build.ResticVersion + ")\n"

	if got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
