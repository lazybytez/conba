// Package logging creates configured zap loggers for conba's diagnostic
// output on stderr.
package logging

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a zap.Logger at the given level. When jsonFormat is true it
// uses a JSON encoder; otherwise a console encoder, colored only when color
// is true. It returns an error if the level string cannot be parsed.
func New(level string, jsonFormat, color bool) (*zap.Logger, error) {
	parsedLevel, err := zapcore.ParseLevel(level)
	if err != nil {
		return nil, fmt.Errorf("parsing log level %q: %w", level, err)
	}

	var zapCfg zap.Config

	if jsonFormat {
		zapCfg = zap.NewProductionConfig()
	} else {
		zapCfg = zap.NewDevelopmentConfig()
		zapCfg.DisableStacktrace = true
		zapCfg.EncoderConfig.EncodeLevel = levelEncoder(color)
	}

	zapCfg.Level = zap.NewAtomicLevelAt(parsedLevel)

	logger, err := zapCfg.Build()
	if err != nil {
		return nil, fmt.Errorf("building logger: %w", err)
	}

	return logger, nil
}

func levelEncoder(color bool) zapcore.LevelEncoder {
	if color {
		return zapcore.CapitalColorLevelEncoder
	}

	return zapcore.CapitalLevelEncoder
}
