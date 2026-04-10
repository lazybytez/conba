package cli_test

import (
	"testing"

	"github.com/lazybytez/conba/internal/cli"
)

func TestNewUnlockCommand_Use(t *testing.T) {
	t.Parallel()

	cmd := cli.NewUnlockCommand()
	if cmd.Use != "unlock" {
		t.Errorf("Use = %q, want %q", cmd.Use, "unlock")
	}
}

func TestNewUnlockCommand_Short(t *testing.T) {
	t.Parallel()

	cmd := cli.NewUnlockCommand()
	if cmd.Short == "" {
		t.Error("Short description must not be empty")
	}
}
