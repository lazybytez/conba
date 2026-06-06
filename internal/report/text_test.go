package report_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/report"
)

const ansiReset = "\x1b[0m"

func TestTextReporter_NoColorIdentical(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	reporter := report.New(report.ModeText, &buf, false)
	reporter.Emit(report.Event{Style: report.StyleSuccess, Message: "Backed up app/data"})

	if got := buf.String(); got != "Backed up app/data\n" {
		t.Errorf("output = %q, want plain line without ANSI", got)
	}
}

func TestTextReporter_ColorWrapsAndStrips(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	reporter := report.New(report.ModeText, &buf, true)
	reporter.Emit(report.Event{Style: report.StyleError, Message: "Failed app/data"})

	got := buf.String()
	if !strings.Contains(got, "\x1b[31m") || !strings.Contains(got, ansiReset) {
		t.Errorf("colored output missing ANSI codes: %q", got)
	}

	if stripped := stripANSI(got); stripped != "Failed app/data\n" {
		t.Errorf("stripped output = %q, want plain line", stripped)
	}
}

func TestTextReporter_ColorizeStyleNoneUnchanged(t *testing.T) {
	t.Parallel()

	reporter := report.New(report.ModeText, &bytes.Buffer{}, true)

	if got := reporter.Colorize(report.StyleNone, "plain"); got != "plain" {
		t.Errorf("StyleNone colorize = %q, want unchanged", got)
	}

	if got := reporter.Colorize(report.StyleSuccess, "ok"); !strings.Contains(got, "\x1b[32m") {
		t.Errorf("StyleSuccess colorize = %q, want green wrap", got)
	}
}

func TestTextReporter_Mode(t *testing.T) {
	t.Parallel()

	if mode := report.New(report.ModeText, &bytes.Buffer{}, false).Mode(); mode != report.ModeText {
		t.Errorf("Mode = %q, want text", mode)
	}
}

func stripANSI(s string) string {
	for _, code := range []string{"\x1b[31m", "\x1b[32m", "\x1b[33m", ansiReset} {
		s = strings.ReplaceAll(s, code, "")
	}

	return s
}
