package cli_test

import "github.com/lazybytez/conba/internal/config"

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
	}
}
