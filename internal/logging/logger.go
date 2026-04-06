// Package logging creates configured zap loggers from conba's logging config.
package logging

import (
	"fmt"

	"github.com/lazybytez/conba/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a zap.Logger from the given logging configuration.
// It returns an error if the level string cannot be parsed.
func New(cfg config.LoggingConfig) (*zap.Logger, error) {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, fmt.Errorf("parsing log level %q: %w", cfg.Level, err)
	}

	var zapCfg zap.Config

	switch cfg.Format {
	case config.LogFormatJSON:
		zapCfg = zap.NewProductionConfig()
	default:
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	zapCfg.Level = zap.NewAtomicLevelAt(level)

	logger, err := zapCfg.Build()
	if err != nil {
		return nil, fmt.Errorf("building logger: %w", err)
	}

	return logger, nil
}
