package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/cli"
	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
	"go.uber.org/zap"
)

func writeMinimalConfig(t *testing.T, dir string) string {
	t.Helper()

	content := []byte(`logging:
  level: info
  format: json
runtime:
  type: docker
  docker:
    host: "unix:///var/run/docker.sock"
`)
	path := filepath.Join(dir, "conba.yaml")

	err := os.WriteFile(path, content, 0o600)
	if err != nil {
		t.Fatalf("write config file: %v", err)
	}

	return path
}

func runPersistentPreRunE(t *testing.T, configPath string) (*config.Config, *zap.Logger, error) {
	t.Helper()

	cmd := cli.NewRootCommand()

	if configPath != "" {
		err := cmd.PersistentFlags().Set("config", configPath)
		if err != nil {
			t.Fatalf("set config flag: %v", err)
		}
	}

	cmd.SetContext(context.Background())

	err := cmd.PersistentPreRunE(cmd, nil)
	if err != nil {
		return nil, nil, err
	}

	cfg := config.FromContext(cmd.Context())
	logger := logging.FromContext(cmd.Context())

	return cfg, logger, nil
}

func TestPersistentPreRunE_LoadsConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := writeMinimalConfig(t, dir)

	cfg, _, err := runPersistentPreRunE(t, cfgPath)
	if err != nil {
		t.Fatalf("PersistentPreRunE returned error: %v", err)
	}

	if cfg == nil {
		t.Fatal("config.FromContext returned nil, want non-nil config")
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("config level = %q, want %q", cfg.Logging.Level, "info")
	}

	if cfg.Logging.Format != "json" {
		t.Errorf("config format = %q, want %q", cfg.Logging.Format, "json")
	}
}

func TestPersistentPreRunE_SetsLogger(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgPath := writeMinimalConfig(t, dir)

	_, logger, err := runPersistentPreRunE(t, cfgPath)
	if err != nil {
		t.Fatalf("PersistentPreRunE returned error: %v", err)
	}

	if logger == nil {
		t.Fatal("logging.FromContext returned nil, want non-nil logger")
	}

	nop := zap.NewNop()
	if logger == nop {
		t.Error("logger is the nop logger, want a real configured logger")
	}
}

func TestPersistentPreRunE_InvalidConfigPath(t *testing.T) {
	t.Parallel()

	_, _, err := runPersistentPreRunE(t, "/nonexistent/path/to/config.yaml")
	if err == nil {
		t.Fatal("PersistentPreRunE returned nil error, want error for invalid config path")
	}

	if !strings.Contains(err.Error(), "load config") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "load config")
	}
}

func TestPersistentPreRunE_DefaultConfig(t *testing.T) {
	t.Parallel()

	cfg, logger, err := runPersistentPreRunE(t, "")
	if err != nil {
		t.Fatalf("PersistentPreRunE returned error: %v", err)
	}

	if cfg == nil {
		t.Fatal("config.FromContext returned nil, want non-nil default config")
	}

	if logger == nil {
		t.Fatal("logging.FromContext returned nil, want non-nil logger")
	}

	if cfg.Logging.Level != "info" {
		t.Errorf("default config level = %q, want %q", cfg.Logging.Level, "info")
	}
}
