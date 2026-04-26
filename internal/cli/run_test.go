package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/cli"
)

func TestNewRunCommand_Use(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRunCommand()

	if cmd.Use != "run" {
		t.Errorf("Use = %q, want %q", cmd.Use, "run")
	}
}

func TestNewRunCommand_Short(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRunCommand()

	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}

	wantSubstrings := []string{"init", "backup", "forget"}
	for _, want := range wantSubstrings {
		if !strings.Contains(cmd.Short, want) {
			t.Errorf("Short %q should mention %q", cmd.Short, want)
		}
	}
}

func TestNewRunCommand_RegistersFlags(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRunCommand()

	boolFlags := []string{"dry-run", "all-hosts", "no-forget"}
	for _, name := range boolFlags {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("flag --%s must exist", name)
		}

		_, err := cmd.Flags().GetBool(name)
		if err != nil {
			t.Errorf("flag --%s should be a bool: %v", name, err)
		}
	}

	for _, name := range boolFlags {
		flag := cmd.Flags().Lookup(name)
		if flag.DefValue != "false" {
			t.Errorf("flag --%s default = %q, want %q", name, flag.DefValue, "false")
		}
	}
}

func TestNewRunCommand_DoesNotRegisterSurgicalFlags(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRunCommand()

	surgicalFlags := []string{"container", "volume", "tag"}
	for _, name := range surgicalFlags {
		if cmd.Flags().Lookup(name) != nil {
			t.Errorf("flag --%s must NOT exist on run; surgical forget flags are forget-only",
				name)
		}
	}
}

func TestNewRunCommand_Help(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRunCommand()

	var buf bytes.Buffer

	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing --help: %v", err)
	}

	output := buf.String()

	wantFlags := []string{"dry-run", "all-hosts", "no-forget"}
	for _, flag := range wantFlags {
		if !strings.Contains(output, "--"+flag) {
			t.Errorf("--help output missing --%s\ngot:\n%s", flag, output)
		}
	}
}

func TestRunRun_NilConfig(t *testing.T) {
	t.Parallel()

	assertRunEFailsWithoutConfig(t, cli.NewRunCommand)
}

func TestRunRun_MissingRepository(t *testing.T) {
	t.Parallel()

	assertRunEFailsWithMissingRepo(t, cli.NewRunCommand)
}
