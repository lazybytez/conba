package report_test

import (
	"testing"

	"github.com/lazybytez/conba/internal/report"
)

func TestResolveCore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		flag      string
		config    string
		tty       bool
		noColor   bool
		wantMode  report.Mode
		wantColor bool
	}{
		{"auto on tty", "", "auto", true, false, report.ModeText, true},
		{"auto off tty", "", "auto", false, false, report.ModeJSON, false},
		{"flag json over tty", "json", "auto", true, false, report.ModeJSON, false},
		{"flag text off tty no color", "text", "auto", false, false, report.ModeText, false},
		{"no_color suppresses color", "", "auto", true, true, report.ModeText, false},
		{"config json", "", "json", true, false, report.ModeJSON, false},
		{"config text off tty", "", "text", false, false, report.ModeText, false},
		{"garbage falls back to auto tty", "x", "y", true, false, report.ModeText, true},
		{"garbage falls back to auto non-tty", "x", "y", false, false, report.ModeJSON, false},
		{"flag wins over config", "text", "json", true, false, report.ModeText, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			mode, color := report.ResolveCore(test.flag, test.config, test.tty, test.noColor)
			if mode != test.wantMode {
				t.Errorf("mode = %q, want %q", mode, test.wantMode)
			}

			if color != test.wantColor {
				t.Errorf("color = %v, want %v", color, test.wantColor)
			}
		})
	}
}
