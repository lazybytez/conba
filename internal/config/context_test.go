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
