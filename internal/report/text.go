package report

import (
	"fmt"
	"io"
)

// ANSI color codes used in text mode.
const (
	ansiReset  = "\x1b[0m"
	ansiRed    = "\x1b[31m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
)

type textReporter struct {
	out   io.Writer
	color bool
}

func newTextReporter(out io.Writer, color bool) *textReporter {
	return &textReporter{out: out, color: color}
}

func (r *textReporter) Mode() Mode { return ModeText }

func (r *textReporter) Out() io.Writer { return r.out }

func (r *textReporter) Emit(event Event) {
	// stdout write failures are not actionable mid-command.
	_, _ = fmt.Fprintln(r.out, r.Colorize(event.Style, event.Message))
}

func (r *textReporter) Colorize(style Style, text string) string {
	if !r.color {
		return text
	}

	code := colorCode(style)
	if code == "" {
		return text
	}

	return code + text + ansiReset
}

func colorCode(style Style) string {
	switch style {
	case StyleSuccess:
		return ansiGreen
	case StyleWarning:
		return ansiYellow
	case StyleError:
		return ansiRed
	case StyleNone:
		return ""
	default:
		return ""
	}
}
