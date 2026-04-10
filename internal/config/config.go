// Package config loads and validates conba's configuration from YAML files,
// environment variables, and built-in defaults using Viper.
package config

import (
	"errors"
	"fmt"
	"regexp"
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

// Supported runtime types.
const (
	RuntimeTypeDocker = "docker"
)

// DefaultDockerHost is the default Docker daemon socket path.
const DefaultDockerHost = "unix:///var/run/docker.sock"

// DefaultResticBinary is the default restic binary name.
const DefaultResticBinary = "restic"

// ErrInvalidLogLevel indicates a log level value that is not supported.
var ErrInvalidLogLevel = errors.New("invalid log level")

// ErrInvalidLogFormat indicates a log format value that is not supported.
var ErrInvalidLogFormat = errors.New("invalid log format")

// ErrInvalidFilterPattern indicates a regex pattern that failed to compile.
var ErrInvalidFilterPattern = errors.New("invalid filter pattern")

// ErrInvalidRuntimeType indicates a runtime type value that is not supported.
var ErrInvalidRuntimeType = errors.New("invalid runtime type")

// ErrMissingRepository indicates restic.repository is not configured.
var ErrMissingRepository = errors.New(
	"restic.repository is required but not configured",
)

// ErrMissingPassword indicates neither restic.password nor
// restic.password_file is configured.
var ErrMissingPassword = errors.New(
	"restic.password or restic.password_file is required but not configured",
)

// Config is the top-level configuration structure for conba.
type Config struct {
	Logging   LoggingConfig   `mapstructure:"logging"`
	Runtime   RuntimeConfig   `mapstructure:"runtime"`
	Discovery DiscoveryConfig `mapstructure:"discovery"`
	Restic    ResticConfig    `mapstructure:"restic"`
}

// ResticConfig holds restic repository and authentication configuration.
type ResticConfig struct {
	Binary       string            `mapstructure:"binary"`
	Repository   string            `mapstructure:"repository"`
	Password     string            `mapstructure:"password"`
	PasswordFile string            `mapstructure:"password_file"`
	ExtraArgs    []string          `mapstructure:"extra_args"`
	Environment  map[string]string `mapstructure:"environment"`
}

// Validate checks that the restic configuration has the minimum required
// fields to interact with a repository. Commands that need restic should
// call this before constructing a client.
func (r ResticConfig) Validate() error {
	if r.Repository == "" {
		return fmt.Errorf(
			"%w: set it in the config file or via CONBA_RESTIC_REPOSITORY",
			ErrMissingRepository,
		)
	}

	if r.Password == "" && r.PasswordFile == "" {
		return fmt.Errorf(
			"%w: set it in the config file or via CONBA_RESTIC_PASSWORD",
			ErrMissingPassword,
		)
	}

	return nil
}

// DiscoveryConfig holds container discovery and filtering settings.
type DiscoveryConfig struct {
	OptInOnly bool       `mapstructure:"opt_in_only"`
	Include   FilterList `mapstructure:"include"`
	Exclude   FilterList `mapstructure:"exclude"`
}

// FilterList holds exact matches and regex patterns for container filtering.
type FilterList struct {
	Names        []string `mapstructure:"names"`
	NamePatterns []string `mapstructure:"name_patterns"`
	IDs          []string `mapstructure:"ids"`
	IDPatterns   []string `mapstructure:"id_patterns"`
}

// RuntimeConfig holds runtime environment configuration.
type RuntimeConfig struct {
	Type   string       `mapstructure:"type"`
	Docker DockerConfig `mapstructure:"docker"`
}

// DockerConfig holds Docker-specific runtime configuration.
type DockerConfig struct {
	Host string `mapstructure:"host"`
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

	err := readConfigFile(viperInstance, cfgFile)
	if err != nil {
		return nil, err
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

func readConfigFile(viperInstance *viper.Viper, cfgFile string) error {
	if cfgFile != "" {
		viperInstance.SetConfigFile(cfgFile)

		err := viperInstance.ReadInConfig()
		if err != nil {
			return fmt.Errorf("reading config: %w", err)
		}

		return nil
	}

	viperInstance.SetConfigName("conba")
	viperInstance.SetConfigType("yaml")
	viperInstance.AddConfigPath(".")
	viperInstance.AddConfigPath("$HOME/.config/conba")
	viperInstance.AddConfigPath("/etc/conba")

	err := viperInstance.ReadInConfig()
	if err == nil {
		return nil
	}

	var lookupErr viper.ConfigFileNotFoundError
	if !errors.As(err, &lookupErr) {
		return fmt.Errorf("reading config: %w", err)
	}

	return nil
}

func setDefaults(viperInstance *viper.Viper) {
	viperInstance.SetDefault("logging.level", LogLevelInfo)
	viperInstance.SetDefault("logging.format", LogFormatHuman)
	viperInstance.SetDefault("runtime.type", RuntimeTypeDocker)
	viperInstance.SetDefault("runtime.docker.host", DefaultDockerHost)
	viperInstance.SetDefault("discovery.opt_in_only", false)
	viperInstance.SetDefault("restic.binary", DefaultResticBinary)
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

	if c.Runtime.Type != RuntimeTypeDocker {
		return fmt.Errorf(
			"%w: %q must be %q",
			ErrInvalidRuntimeType,
			c.Runtime.Type,
			RuntimeTypeDocker,
		)
	}

	err := validateFilterPatterns(c.Discovery.Include)
	if err != nil {
		return fmt.Errorf("discovery.include: %w", err)
	}

	err = validateFilterPatterns(c.Discovery.Exclude)
	if err != nil {
		return fmt.Errorf("discovery.exclude: %w", err)
	}

	return nil
}

func validateFilterPatterns(list FilterList) error {
	for _, pattern := range list.NamePatterns {
		_, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf(
				"%w: name_patterns %q: %w",
				ErrInvalidFilterPattern, pattern, err,
			)
		}
	}

	for _, pattern := range list.IDPatterns {
		_, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf(
				"%w: id_patterns %q: %w",
				ErrInvalidFilterPattern, pattern, err,
			)
		}
	}

	return nil
}
