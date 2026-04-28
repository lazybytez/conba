package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/cli"
)

func TestNewVerifyCommand_Use(t *testing.T) {
	t.Parallel()

	cmd := cli.NewVerifyCommand()
	if cmd.Use != "verify" {
		t.Errorf("Use = %q, want %q", cmd.Use, "verify")
	}
}

func TestNewVerifyCommand_Short(t *testing.T) {
	t.Parallel()

	cmd := cli.NewVerifyCommand()
	if cmd.Short == "" {
		t.Error("Short description must not be empty")
	}
}

func TestNewVerifyCommand_RegistersReadDataFlag(t *testing.T) {
	t.Parallel()

	cmd := cli.NewVerifyCommand()

	flag := cmd.Flags().Lookup("read-data")
	if flag == nil {
		t.Fatal("flag --read-data must exist")
	}

	if flag.DefValue != "false" {
		t.Errorf("flag --read-data default = %q, want %q", flag.DefValue, "false")
	}
}

func TestNewVerifyCommand_Help(t *testing.T) {
	t.Parallel()

	cmd := cli.NewVerifyCommand()

	var buf bytes.Buffer

	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing --help: %v", err)
	}

	if !strings.Contains(buf.String(), "--read-data") {
		t.Errorf("--help output missing --read-data\ngot:\n%s", buf.String())
	}
}

func TestRunVerify_NilConfig(t *testing.T) {
	t.Parallel()
	assertRunEFailsWithoutConfig(t, cli.NewVerifyCommand)
}

func TestRunVerify_MissingRepository(t *testing.T) {
	t.Parallel()
	assertRunEFailsWithMissingRepo(t, cli.NewVerifyCommand)
}
