package restic_test

import (
	"slices"
	"testing"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/restic"
)

func testResticConfig(repo string, opts ...func(*config.ResticConfig)) config.ResticConfig {
	cfg := config.ResticConfig{
		Binary:       "",
		Repository:   repo,
		Password:     "",
		PasswordFile: "",
		ExtraArgs:    nil,
		Environment:  nil,
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return cfg
}

func withPassword(pw string) func(*config.ResticConfig) {
	return func(cfg *config.ResticConfig) {
		cfg.Password = pw
	}
}

func withPasswordFile(path string) func(*config.ResticConfig) {
	return func(cfg *config.ResticConfig) {
		cfg.PasswordFile = path
	}
}

func withEnvironment(env map[string]string) func(*config.ResticConfig) {
	return func(cfg *config.ResticConfig) {
		cfg.Environment = env
	}
}

func TestBuildEnv_PasswordHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  config.ResticConfig
		want []string
	}{
		{
			name: "password set without password_file",
			cfg:  testResticConfig("/repo", withPassword("secret")),
			want: []string{
				"RESTIC_PASSWORD=secret",
				"RESTIC_REPOSITORY=/repo",
			},
		},
		{
			name: "password_file takes priority",
			cfg: testResticConfig(
				"/repo",
				withPassword("secret"),
				withPasswordFile("/etc/restic/pass"),
			),
			want: []string{
				"RESTIC_PASSWORD_FILE=/etc/restic/pass",
				"RESTIC_REPOSITORY=/repo",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildEnv(test.cfg)
			if !slices.Equal(got, test.want) {
				t.Errorf("BuildEnv() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestBuildEnv_Environment(t *testing.T) {
	t.Parallel()

	cfg := testResticConfig("/repo", withPassword("secret"), withEnvironment(map[string]string{
		"b_custom_var":      "two",
		"aws_access_key_id": "AKID",
	}))

	got := restic.BuildEnv(cfg)
	want := []string{
		"AWS_ACCESS_KEY_ID=AKID",
		"B_CUSTOM_VAR=two",
		"RESTIC_PASSWORD=secret",
		"RESTIC_REPOSITORY=/repo",
	}

	if !slices.Equal(got, want) {
		t.Errorf("BuildEnv() = %v, want %v", got, want)
	}
}

func TestBuildEnv_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  config.ResticConfig
		want []string
	}{
		{
			name: "neither password nor password_file",
			cfg:  testResticConfig("/repo"),
			want: []string{
				"RESTIC_REPOSITORY=/repo",
			},
		},
		{
			name: "empty repository still included",
			cfg:  testResticConfig(""),
			want: []string{
				"RESTIC_REPOSITORY=",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.BuildEnv(test.cfg)
			if !slices.Equal(got, test.want) {
				t.Errorf("BuildEnv() = %v, want %v", got, test.want)
			}
		})
	}
}
