package report

import (
	"os"
	"strings"
)

// Mode is the output rendering mode.
type Mode string

// Supported rendering modes.
const (
	ModeText Mode = "text"
	ModeJSON Mode = "json"
)

// Resolve determines the output mode and whether color is enabled from the
// --output flag value, the configured output.format, and stdout.
//
// Precedence: an explicit "text"/"json" flag wins, then config; anything
// else ("auto" or empty) resolves to text on a terminal and json otherwise.
// Color is enabled only in text mode, on a terminal, with NO_COLOR unset.
func Resolve(flagValue, configValue string, stdout *os.File) (Mode, bool) {
	_, noColor := os.LookupEnv("NO_COLOR")

	return resolve(flagValue, configValue, isTerminal(stdout), noColor)
}

// resolve is the testable core of Resolve with terminal and NO_COLOR state
// passed in explicitly.
func resolve(flagValue, configValue string, tty, noColor bool) (Mode, bool) {
	mode := resolveMode(flagValue, configValue, tty)
	color := mode == ModeText && tty && !noColor

	return mode, color
}

func resolveMode(flagValue, configValue string, tty bool) Mode {
	for _, value := range []string{flagValue, configValue} {
		switch strings.ToLower(value) {
		case string(ModeText):
			return ModeText
		case string(ModeJSON):
			return ModeJSON
		}
	}

	if tty {
		return ModeText
	}

	return ModeJSON
}

func isTerminal(file *os.File) bool {
	if file == nil {
		return false
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}
