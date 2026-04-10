package cli_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/cli"
	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
	"go.uber.org/zap"
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

func TestRunInit_NilConfig(t *testing.T) {
	t.Parallel()

	cmd := cli.NewInitCommand()
	cmd.SetContext(context.Background())

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("want error, got nil")
	}
}

func TestRunInit_MissingRepository(t *testing.T) {
	t.Parallel()

	cfg := testConfigWithRestic(config.ResticConfig{
		Binary:       "restic",
		Repository:   "",
		Password:     "secret",
		PasswordFile: "",
		ExtraArgs:    nil,
		Environment:  nil,
	})

	ctx := config.WithConfig(context.Background(), cfg)
	ctx = logging.WithLogger(ctx, zap.NewNop())

	cmd := cli.NewInitCommand()
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(err, config.ErrMissingRepository) {
		t.Errorf("want ErrMissingRepository, got %v", err)
	}
}
