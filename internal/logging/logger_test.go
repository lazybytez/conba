package logging_test

import (
	"testing"

	"github.com/lazybytez/conba/internal/logging"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		level      string
		jsonFormat bool
		color      bool
		wantErr    bool
	}{
		{
			name: "text info", level: "info",
			jsonFormat: false, color: false, wantErr: false,
		},
		{
			name: "text debug colored", level: "debug",
			jsonFormat: false, color: true, wantErr: false,
		},
		{
			name: "json info", level: "info",
			jsonFormat: true, color: false, wantErr: false,
		},
		{
			name: "invalid level", level: "trace",
			jsonFormat: false, color: false, wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			logger, err := logging.New(test.level, test.jsonFormat, test.color)

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
