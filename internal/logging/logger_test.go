package logging_test

import (
	"testing"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/logging"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     config.LoggingConfig
		wantErr bool
	}{
		{
			name:    "human info",
			cfg:     config.LoggingConfig{Level: "info", Format: "human"},
			wantErr: false,
		},
		{
			name:    "human debug",
			cfg:     config.LoggingConfig{Level: "debug", Format: "human"},
			wantErr: false,
		},
		{
			name:    "json info",
			cfg:     config.LoggingConfig{Level: "info", Format: "json"},
			wantErr: false,
		},
		{
			name:    "invalid level",
			cfg:     config.LoggingConfig{Level: "trace", Format: "human"},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			logger, err := logging.New(test.cfg)

			if test.wantErr {
				if err == nil {
					t.Fatal("New() expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("New() unexpected error: %v", err)
			}

			if logger == nil {
				t.Fatal("New() returned nil logger")
			}
		})
	}
}
