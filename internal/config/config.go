// Package config loads and validates conba's configuration from YAML files,
// environment variables, and built-in defaults using Viper.
package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// ErrInvalidLogLevel indicates a log level value that is not supported.
var ErrInvalidLogLevel = errors.New("invalid log level")

// ErrInvalidLogFormat indicates a log format value that is not supported.
var ErrInvalidLogFormat = errors.New("invalid log format")

// Config is the top-level configuration structure for conba.
type Config struct {
	Logging LoggingConfig `mapstructure:"logging"`
}

// LoggingConfig holds logging-related configuration values.
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// Load reads configuration from the given file path (if non-empty),
// environment variables, and built-in defaults. It returns the validated
// configuration or an error.
func Load(cfgFile string) (*Config, error) {
	viperInstance := viper.New()

	viperInstance.SetDefault("logging.level", "info")
	viperInstance.SetDefault("logging.format", "human")

	viperInstance.SetEnvPrefix("CONBA")
	viperInstance.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viperInstance.AutomaticEnv()

	if cfgFile != "" {
		viperInstance.SetConfigFile(cfgFile)
	} else {
		viperInstance.SetConfigName("conba")
		viperInstance.SetConfigType("yaml")
		viperInstance.AddConfigPath(".")
		viperInstance.AddConfigPath("$HOME/.config/conba")
		viperInstance.AddConfigPath("/etc/conba")
	}

	err := viperInstance.ReadInConfig()
	if err != nil {
		var lookupErr viper.ConfigFileNotFoundError
		if cfgFile != "" || !errors.As(err, &lookupErr) {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var cfg Config

	err = viperInstance.Unmarshal(&cfg)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	err = cfg.validate()
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	switch c.Logging.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf(
			"%w: %q must be one of debug, info, warn, error",
			ErrInvalidLogLevel,
			c.Logging.Level,
		)
	}

	switch c.Logging.Format {
	case "human", "json":
	default:
		return fmt.Errorf(
			"%w: %q must be one of human, json",
			ErrInvalidLogFormat,
			c.Logging.Format,
		)
	}

	return nil
}
