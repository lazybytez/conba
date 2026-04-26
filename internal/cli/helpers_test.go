package cli_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func testConfigWithRestic(resticCfg config.ResticConfig) *config.Config {
	return &config.Config{
		Logging: config.LoggingConfig{
			Level:  config.LogLevelInfo,
			Format: config.LogFormatHuman,
		},
		Runtime: config.RuntimeConfig{
			Type: config.RuntimeTypeDocker,
			Docker: config.DockerConfig{
				Host: "unix:///var/run/docker.sock",
			},
		},
		Discovery: config.DiscoveryConfig{
			OptInOnly: false,
			Include: config.FilterList{
				Names:        nil,
				NamePatterns: nil,
				IDs:          nil,
				IDPatterns:   nil,
			},
			Exclude: config.FilterList{
				Names:        nil,
				NamePatterns: nil,
				IDs:          nil,
				IDPatterns:   nil,
			},
		},
		Restic: resticCfg,
		Retention: config.RetentionConfig{
			KeepDaily:   0,
			KeepWeekly:  0,
			KeepMonthly: 0,
			KeepYearly:  0,
		},
	}
}

func assertRunEFailsWithoutConfig(t *testing.T, newCmd func() *cobra.Command) {
	t.Helper()

	cmd := newCmd()
	cmd.SetContext(context.Background())

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("want error for nil config, got nil")
	}
}

func assertRunEFailsWithMissingRepo(t *testing.T, newCmd func() *cobra.Command) {
	t.Helper()

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

	cmd := newCmd()
	cmd.SetContext(ctx)

	err := cmd.RunE(cmd, nil)
	if err == nil {
		t.Fatal("want error for missing repository, got nil")
	}

	if !errors.Is(err, config.ErrMissingRepository) {
		t.Errorf("want ErrMissingRepository, got %v", err)
	}
}
