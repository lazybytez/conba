package report

import (
	"encoding/json"
	"io"
	"time"
)

// coreJSONKeys is the number of always-present keys (time, level, event)
// pre-sized into each record map alongside the event's own fields.
const coreJSONKeys = 3

type jsonReporter struct {
	encoder *json.Encoder
	out     io.Writer
	now     func() time.Time
}

func newJSONReporter(out io.Writer) *jsonReporter {
	return &jsonReporter{
		encoder: json.NewEncoder(out),
		out:     out,
		now:     time.Now,
	}
}

func (r *jsonReporter) Mode() Mode { return ModeJSON }

func (r *jsonReporter) Out() io.Writer { return r.out }

func (r *jsonReporter) Colorize(_ Style, text string) string { return text }

func (r *jsonReporter) Emit(event Event) {
	level := event.Level
	if level == "" {
		level = LevelInfo
	}

	record := make(map[string]any, len(event.Fields)+coreJSONKeys)
	record["time"] = r.now().UTC().Format(time.RFC3339)
	record["level"] = string(level)
	record["event"] = event.Name

	for _, field := range event.Fields {
		record[field.Key] = field.Value
	}

	// Best-effort: stdout write/encode failures are not actionable
	// mid-command, and the exit code still reflects the operation outcome.
	//nolint:errchkjson // record holds only command-supplied JSON-safe values.
	_ = r.encoder.Encode(record)
}
