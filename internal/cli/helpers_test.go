package cli_test

import (
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/cli"
	"github.com/lazybytez/conba/internal/config"
)

func TestRequireResticConfig_Valid(t *testing.T) {
	t.Parallel()

	cfg := config.ResticConfig{
		Binary:       "restic",
		Repository:   "/tmp/repo",
		Password:     "secret",
		PasswordFile: "",
		ExtraArgs:    nil,
		Environment:  nil,
	}

	err := cli.RequireResticConfig(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRequireResticConfig_PasswordFileValid(t *testing.T) {
	t.Parallel()

	cfg := config.ResticConfig{
		Binary:       "restic",
		Repository:   "/tmp/repo",
		Password:     "",
		PasswordFile: "/etc/restic/pass",
		ExtraArgs:    nil,
		Environment:  nil,
	}

	err := cli.RequireResticConfig(cfg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRequireResticConfig_MissingRepository(t *testing.T) {
	t.Parallel()

	cfg := config.ResticConfig{
		Binary:       "restic",
		Repository:   "",
		Password:     "secret",
		PasswordFile: "",
		ExtraArgs:    nil,
		Environment:  nil,
	}

	err := cli.RequireResticConfig(cfg)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(err, cli.ErrMissingRepository) {
		t.Errorf("want ErrMissingRepository, got %v", err)
	}
}

func TestRequireResticConfig_MissingPassword(t *testing.T) {
	t.Parallel()

	cfg := config.ResticConfig{
		Binary:       "restic",
		Repository:   "/tmp/repo",
		Password:     "",
		PasswordFile: "",
		ExtraArgs:    nil,
		Environment:  nil,
	}

	err := cli.RequireResticConfig(cfg)
	if err == nil {
		t.Fatal("want error, got nil")
	}

	if !errors.Is(err, cli.ErrMissingPassword) {
		t.Errorf("want ErrMissingPassword, got %v", err)
	}
}
