package report

import "io"

// Reporter renders command output as either human text or JSON events.
//
// Emit handles the common case (one line / one object). List commands that
// need aligned text tables or richly typed JSON rows can branch on Mode and
// write through Out, using Colorize for any color.
type Reporter interface {
	// Emit renders one event.
	Emit(event Event)
	// Mode reports the active render mode.
	Mode() Mode
	// Out returns the destination writer, for table rendering in text mode.
	Out() io.Writer
	// Colorize wraps s in the ANSI color for style when color is enabled,
	// and returns s unchanged otherwise (always in JSON mode).
	Colorize(style Style, text string) string
}

// New builds a Reporter for the given mode writing to out. Color applies
// only in text mode and only when color is true.
//
//nolint:ireturn // factory returns the Reporter abstraction by design.
func New(mode Mode, out io.Writer, color bool) Reporter {
	if mode == ModeJSON {
		return newJSONReporter(out)
	}

	return newTextReporter(out, color)
}

// Nop returns a reporter that discards all output. It is the zero-value
// reporter returned by FromContext when none is set.
//
//nolint:ireturn // returns the Reporter abstraction by design.
func Nop() Reporter {
	return newTextReporter(io.Discard, false)
}
