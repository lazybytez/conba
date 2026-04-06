// Package config loads and validates conba's configuration from YAML files,
// environment variables, and built-in defaults using Viper.
package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Supported log levels.
const (
	LogLevelDebug = "debug"
	LogLevelInfo  = "info"
	LogLevelWarn  = "warn"
	LogLevelError = "error"
)

// Supported log formats.
const (
	LogFormatHuman = "human"
	LogFormatJSON  = "json"
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

	setDefaults(viperInstance)

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

func setDefaults(v *viper.Viper) {
	v.SetDefault("logging.level", LogLevelInfo)
	v.SetDefault("logging.format", LogFormatHuman)
}

func (c *Config) validate() error {
	switch c.Logging.Level {
	case LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError:
	default:
		return fmt.Errorf(
			"%w: %q must be one of %s, %s, %s, %s",
			ErrInvalidLogLevel,
			c.Logging.Level,
			LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError,
		)
	}

	switch c.Logging.Format {
	case LogFormatHuman, LogFormatJSON:
	default:
		return fmt.Errorf(
			"%w: %q must be one of %s, %s",
			ErrInvalidLogFormat,
			c.Logging.Format,
			LogFormatHuman, LogFormatJSON,
		)
	}

	return nil
}
