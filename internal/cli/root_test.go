package cli_test

import (
	"testing"

	"github.com/lazybytez/conba/internal/cli"
)

func TestNewRootCommand_Use(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCommand()
	if cmd.Use != "conba" {
		t.Errorf("Use = %q, want %q", cmd.Use, "conba")
	}
}

func TestNewRootCommand_Short(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCommand()
	if cmd.Short == "" {
		t.Error("Short description must not be empty")
	}
}

func TestNewRootCommand_Long(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCommand()
	if cmd.Long == "" {
		t.Error("Long description must not be empty")
	}
}

func TestNewRootCommand_SilenceUsage(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCommand()
	if !cmd.SilenceUsage {
		t.Error("SilenceUsage must be true")
	}
}

func TestNewRootCommand_SilenceErrors(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCommand()
	if !cmd.SilenceErrors {
		t.Error("SilenceErrors must be true")
	}
}

func TestNewRootCommand_HasConfigFlag(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCommand()

	flag := cmd.PersistentFlags().Lookup("config")
	if flag == nil {
		t.Fatal("persistent flag --config must exist")
	}

	if flag.Shorthand != "c" {
		t.Errorf("config shorthand = %q, want %q", flag.Shorthand, "c")
	}

	if flag.DefValue != "" {
		t.Errorf("config default = %q, want empty string", flag.DefValue)
	}
}

func TestNewRootCommand_HasVersionSubcommand(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCommand()

	var found bool

	for _, sub := range cmd.Commands() {
		if sub.Use == "version" {
			found = true

			break
		}
	}

	if !found {
		t.Error("root command must have a version subcommand")
	}
}

func TestNewRootCommand_PersistentPreRunE_IsSet(t *testing.T) {
	t.Parallel()

	cmd := cli.NewRootCommand()
	if cmd.PersistentPreRunE == nil {
		t.Error("PersistentPreRunE must be set for config loading")
	}
}

func TestExecute_ReturnsNoError(t *testing.T) {
	t.Parallel()

	err := cli.Execute()
	if err != nil {
		t.Errorf("Execute() returned error: %v", err)
	}
}
