package cli_test

import (
	"testing"

	"github.com/lazybytez/conba/internal/cli"
)

func TestNewInitCommand_Use(t *testing.T) {
	t.Parallel()

	cmd := cli.NewInitCommand()
	if cmd.Use != "init" {
		t.Errorf("Use = %q, want %q", cmd.Use, "init")
	}
}

func TestNewInitCommand_Short(t *testing.T) {
	t.Parallel()

	cmd := cli.NewInitCommand()
	if cmd.Short == "" {
		t.Error("Short description must not be empty")
	}
}
