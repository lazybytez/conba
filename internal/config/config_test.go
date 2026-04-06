package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lazybytez/conba/internal/config"
)

func TestLoadDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.Logging.Level != config.LogLevelInfo {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, config.LogLevelInfo)
	}

	if cfg.Logging.Format != config.LogFormatHuman {
		t.Errorf("Logging.Format = %q, want %q", cfg.Logging.Format, config.LogFormatHuman)
	}

	if cfg.Runtime.Type != config.RuntimeTypeDocker {
		t.Errorf("Runtime.Type = %q, want %q", cfg.Runtime.Type, config.RuntimeTypeDocker)
	}

	if cfg.Runtime.Docker.Host != config.DefaultDockerHost {
		t.Errorf("Runtime.Docker.Host = %q, want %q",
			cfg.Runtime.Docker.Host, config.DefaultDockerHost)
	}
}

func TestLoadFromYAMLFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "conba.yaml")
	content := []byte("logging:\n  level: debug\n  format: json\n")

	writeErr := os.WriteFile(cfgFile, content, 0o600)
	if writeErr != nil {
		t.Fatalf("failed to write temp config: %v", writeErr)
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.Logging.Level != config.LogLevelDebug {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, config.LogLevelDebug)
	}

	if cfg.Logging.Format != config.LogFormatJSON {
		t.Errorf("Logging.Format = %q, want %q", cfg.Logging.Format, config.LogFormatJSON)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	t.Setenv("CONBA_LOGGING_LEVEL", config.LogLevelDebug)

	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.Logging.Level != config.LogLevelDebug {
		t.Errorf("Logging.Level = %q, want %q", cfg.Logging.Level, config.LogLevelDebug)
	}

	if cfg.Logging.Format != config.LogFormatHuman {
		t.Errorf("Logging.Format = %q, want %q", cfg.Logging.Format, config.LogFormatHuman)
	}
}

func TestLoadValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		wantErr error
	}{
		{
			name:    "invalid level trace",
			yaml:    "logging:\n  level: trace\n  format: json\n",
			wantErr: config.ErrInvalidLogLevel,
		},
		{
			name:    "invalid level fatal",
			yaml:    "logging:\n  level: fatal\n  format: json\n",
			wantErr: config.ErrInvalidLogLevel,
		},
		{
			name:    "invalid format text",
			yaml:    "logging:\n  level: info\n  format: text\n",
			wantErr: config.ErrInvalidLogFormat,
		},
		{
			name:    "invalid format xml",
			yaml:    "logging:\n  level: info\n  format: xml\n",
			wantErr: config.ErrInvalidLogFormat,
		},
		{
			name:    "invalid runtime type podman",
			yaml:    "runtime:\n  type: podman\n",
			wantErr: config.ErrInvalidRuntimeType,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			cfgFile := filepath.Join(dir, "conba.yaml")

			writeErr := os.WriteFile(cfgFile, []byte(test.yaml), 0o600)
			if writeErr != nil {
				t.Fatalf("failed to write temp config: %v", writeErr)
			}

			_, err := config.Load(cfgFile)
			if err == nil {
				t.Fatal("Load() expected error, got nil")
			}

			if !errors.Is(err, test.wantErr) {
				t.Errorf("error = %q, want %v", err.Error(), test.wantErr)
			}
		})
	}
}

func TestLoadExplicitMissingFile(t *testing.T) {
	t.Parallel()

	_, err := config.Load("/nonexistent/conba.yaml")
	if err == nil {
		t.Fatal("Load() expected error for nonexistent explicit file, got nil")
	}
}
