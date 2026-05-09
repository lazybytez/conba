package config_test

import (
	"context"
	"testing"

	"github.com/lazybytez/conba/internal/config"
)

func TestWithConfigAndFromContext(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logging: config.LoggingConfig{Level: config.LogLevelInfo, Format: config.LogFormatHuman},
		Runtime: config.RuntimeConfig{
			Type:   config.RuntimeTypeDocker,
			Docker: config.DockerConfig{Host: ""},
		},
		Discovery: config.DiscoveryConfig{
			OptInOnly: false,
			Include:   config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
			Exclude:   config.FilterList{Names: nil, NamePatterns: nil, IDs: nil, IDPatterns: nil},
		},
		Restic: config.ResticConfig{
			Binary:       "",
			Repository:   "",
			Password:     "",
			PasswordFile: "",
			ExtraArgs:    nil,
			Environment:  nil,
		},
		Retention: config.RetentionConfig{
			KeepDaily:   0,
			KeepWeekly:  0,
			KeepMonthly: 0,
			KeepYearly:  0,
		},
	}
	ctx := config.WithConfig(context.Background(), cfg)

	got := config.FromContext(ctx)
	if got != cfg {
		t.Error("FromContext did not return the config set by WithConfig")
	}
}

func TestFromContext_ReturnsNilWhenMissing(t *testing.T) {
	t.Parallel()

	got := config.FromContext(context.Background())
	if got != nil {
		t.Error("FromContext returned non-nil, want nil")
	}
}
