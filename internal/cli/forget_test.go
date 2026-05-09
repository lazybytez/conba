package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/cli"
)

func TestNewForgetCommand_Use(t *testing.T) {
	t.Parallel()

	cmd := cli.NewForgetCommand()

	if cmd.Use != "forget" {
		t.Errorf("Use = %q, want %q", cmd.Use, "forget")
	}
}

func TestNewForgetCommand_Short(t *testing.T) {
	t.Parallel()

	cmd := cli.NewForgetCommand()

	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
}

func TestNewForgetCommand_RegistersFlags(t *testing.T) {
	t.Parallel()

	cmd := cli.NewForgetCommand()

	boolFlags := []string{"dry-run", "no-prune", "all-hosts"}
	for _, name := range boolFlags {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("flag --%s must exist", name)
		}

		_, err := cmd.Flags().GetBool(name)
		if err != nil {
			t.Errorf("flag --%s should be a bool: %v", name, err)
		}
	}

	stringFlags := []string{"container", "volume"}
	for _, name := range stringFlags {
		if cmd.Flags().Lookup(name) == nil {
			t.Fatalf("flag --%s must exist", name)
		}

		_, err := cmd.Flags().GetString(name)
		if err != nil {
			t.Errorf("flag --%s should be a string: %v", name, err)
		}
	}

	if cmd.Flags().Lookup("tag") == nil {
		t.Fatal("flag --tag must exist")
	}

	_, err := cmd.Flags().GetStringArray("tag")
	if err != nil {
		t.Errorf("flag --tag should be a string array: %v", err)
	}
}

func TestNewForgetCommand_Help(t *testing.T) {
	t.Parallel()

	cmd := cli.NewForgetCommand()

	var buf bytes.Buffer

	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing --help: %v", err)
	}

	output := buf.String()

	wantFlags := []string{"dry-run", "no-prune", "all-hosts", "container", "volume", "tag"}
	for _, flag := range wantFlags {
		if !strings.Contains(output, "--"+flag) {
			t.Errorf("--help output missing --%s\ngot:\n%s", flag, output)
		}
	}
}

func TestRunForget_NilConfig(t *testing.T) {
	t.Parallel()

	assertRunEFailsWithoutConfig(t, cli.NewForgetCommand)
}
