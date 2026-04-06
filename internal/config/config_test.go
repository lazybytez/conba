package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
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

	if cfg.Discovery.OptInOnly {
		t.Errorf("Discovery.OptInOnly = %v, want %v", cfg.Discovery.OptInOnly, false)
	}

	if len(cfg.Discovery.Include.Names) != 0 {
		t.Errorf("Discovery.Include.Names = %v, want empty", cfg.Discovery.Include.Names)
	}

	if len(cfg.Discovery.Exclude.Names) != 0 {
		t.Errorf("Discovery.Exclude.Names = %v, want empty", cfg.Discovery.Exclude.Names)
	}

	t.Run("restic defaults", func(t *testing.T) {
		t.Parallel()
		assertResticDefaults(t, cfg.Restic)
	})
}

func assertResticDefaults(
	t *testing.T,
	restic config.ResticConfig,
) {
	t.Helper()

	if restic.Binary != config.DefaultResticBinary {
		t.Errorf("Binary = %q, want %q", restic.Binary, config.DefaultResticBinary)
	}

	if restic.Repository != "" {
		t.Errorf("Repository = %q, want %q", restic.Repository, "")
	}

	if restic.Password != "" {
		t.Errorf("Password = %q, want %q", restic.Password, "")
	}

	if restic.PasswordFile != "" {
		t.Errorf("PasswordFile = %q, want %q", restic.PasswordFile, "")
	}

	if len(restic.ExtraArgs) != 0 {
		t.Errorf("ExtraArgs = %v, want empty", restic.ExtraArgs)
	}

	if len(restic.Environment) != 0 {
		t.Errorf("Environment = %v, want empty", restic.Environment)
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

func TestLoadValidation_InvalidFilterPatterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "invalid include name pattern",
			yaml: "discovery:\n  include:\n    name_patterns:\n      - \"[\"\n",
		},
		{
			name: "invalid exclude id pattern",
			yaml: "discovery:\n  exclude:\n    id_patterns:\n      - \"[\"\n",
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

			if !errors.Is(err, config.ErrInvalidFilterPattern) {
				t.Errorf("error = %q, want %v", err.Error(), config.ErrInvalidFilterPattern)
			}
		})
	}
}

func TestLoadDiscoveryFromYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "conba.yaml")
	content := []byte(`discovery:
  opt_in_only: true
  include:
    names: ["myapp", "postgres"]
    ids: ["abc123"]
  exclude:
    names: ["redis"]
`)

	writeErr := os.WriteFile(cfgFile, content, 0o600)
	if writeErr != nil {
		t.Fatalf("failed to write temp config: %v", writeErr)
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if !cfg.Discovery.OptInOnly {
		t.Error("Discovery.OptInOnly = false, want true")
	}

	if !slices.Equal(cfg.Discovery.Include.Names, []string{"myapp", "postgres"}) {
		t.Errorf("Include.Names = %v", cfg.Discovery.Include.Names)
	}

	if !slices.Equal(cfg.Discovery.Include.IDs, []string{"abc123"}) {
		t.Errorf("Include.IDs = %v", cfg.Discovery.Include.IDs)
	}

	if !slices.Equal(cfg.Discovery.Exclude.Names, []string{"redis"}) {
		t.Errorf("Exclude.Names = %v", cfg.Discovery.Exclude.Names)
	}

	if len(cfg.Discovery.Exclude.IDs) != 0 {
		t.Errorf("Exclude.IDs = %v, want empty", cfg.Discovery.Exclude.IDs)
	}
}

func TestLoadResticFromYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "conba.yaml")
	content := []byte(`restic:
  binary: /usr/bin/restic
  repository: "/tmp/restic-test-repo"
  password: "secret"
  password_file: "/run/secrets/restic-pw"
  extra_args: ["--verbose"]
  environment:
    RESTIC_CACHE_DIR: "/tmp/restic-cache"
`)

	writeErr := os.WriteFile(cfgFile, content, 0o600)
	if writeErr != nil {
		t.Fatalf("failed to write temp config: %v", writeErr)
	}

	cfg, err := config.Load(cfgFile)
	if err != nil {
		t.Fatalf("Load() returned unexpected error: %v", err)
	}

	if cfg.Restic.Binary != "/usr/bin/restic" {
		t.Errorf("Restic.Binary = %q, want %q", cfg.Restic.Binary, "/usr/bin/restic")
	}

	if cfg.Restic.Repository != "/tmp/restic-test-repo" {
		t.Errorf(
			"Restic.Repository = %q, want %q",
			cfg.Restic.Repository,
			"/tmp/restic-test-repo",
		)
	}

	if cfg.Restic.Password != "secret" {
		t.Errorf("Restic.Password = %q, want %q", cfg.Restic.Password, "secret")
	}

	if cfg.Restic.PasswordFile != "/run/secrets/restic-pw" {
		t.Errorf(
			"Restic.PasswordFile = %q, want %q",
			cfg.Restic.PasswordFile,
			"/run/secrets/restic-pw",
		)
	}

	if !slices.Equal(cfg.Restic.ExtraArgs, []string{"--verbose"}) {
		t.Errorf("Restic.ExtraArgs = %v, want %v", cfg.Restic.ExtraArgs, []string{"--verbose"})
	}

	assertResticEnvironment(t, cfg.Restic.Environment, map[string]string{
		"restic_cache_dir": "/tmp/restic-cache",
	})
}

func assertResticEnvironment(
	t *testing.T,
	got map[string]string,
	want map[string]string,
) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf(
			"Environment has %d entries, want %d",
			len(got),
			len(want),
		)
	}

	for key, wantVal := range want {
		gotVal, ok := got[key]
		if !ok {
			t.Errorf("Environment missing key %q", key)
		}

		if gotVal != wantVal {
			t.Errorf(
				"Environment[%q] = %q, want %q",
				key,
				gotVal,
				wantVal,
			)
		}
	}
}

func TestLoadExplicitMissingFile(t *testing.T) {
	t.Parallel()

	_, err := config.Load("/nonexistent/conba.yaml")
	if err == nil {
		t.Fatal("Load() expected error for nonexistent explicit file, got nil")
	}
}
