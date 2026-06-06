package report_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/lazybytez/conba/internal/report"
)

func TestJSONReporter_Emit(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	reporter := report.New(report.ModeJSON, &buf, false)
	reporter.Emit(report.Event{
		Level: report.LevelWarn,
		Name:  "backup.target",
		Fields: []report.Field{
			report.F("container", "mysql"),
			report.F("outcome", "skipped"),
		},
	})

	record := decodeOne(t, buf.String())

	if record["event"] != "backup.target" {
		t.Errorf("event = %v, want backup.target", record["event"])
	}

	if record["level"] != "warn" {
		t.Errorf("level = %v, want warn", record["level"])
	}

	if record["container"] != "mysql" || record["outcome"] != "skipped" {
		t.Errorf("fields not carried: %v", record)
	}

	timeStr, ok := record["time"].(string)
	if !ok {
		t.Fatalf("time missing or not a string: %v", record["time"])
	}

	_, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		t.Errorf("time %q not RFC3339: %v", timeStr, err)
	}
}

func TestJSONReporter_DefaultLevel(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	report.New(report.ModeJSON, &buf, false).Emit(report.Event{Name: "init.done"})

	if level := decodeOne(t, buf.String())["level"]; level != "info" {
		t.Errorf("default level = %v, want info", level)
	}
}

func TestJSONReporter_NoANSI(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	reporter := report.New(report.ModeJSON, &buf, false)
	reporter.Emit(report.Event{Style: report.StyleError, Name: "x", Message: "boom"})

	if strings.Contains(buf.String(), "\x1b[") {
		t.Errorf("json output contains ANSI escape: %q", buf.String())
	}

	if got := reporter.Colorize(report.StyleError, "boom"); got != "boom" {
		t.Errorf("Colorize in json mode = %q, want unchanged", got)
	}
}

func TestJSONReporter_Mode(t *testing.T) {
	t.Parallel()

	if mode := report.New(report.ModeJSON, &bytes.Buffer{}, false).Mode(); mode != report.ModeJSON {
		t.Errorf("Mode = %q, want json", mode)
	}
}

func decodeOne(t *testing.T, line string) map[string]any {
	t.Helper()

	record := map[string]any{}

	err := json.Unmarshal([]byte(strings.TrimSpace(line)), &record)
	if err != nil {
		t.Fatalf("output is not valid JSON (%v): %q", err, line)
	}

	return record
}
