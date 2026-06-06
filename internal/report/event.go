// Package report renders command output as either human-readable text or a
// newline-delimited JSON event stream, chosen per invocation. Commands and
// orchestrators emit structured events through a Reporter rather than writing
// to an io.Writer directly, so the same call serves an interactive operator
// and an automated consumer scraping logs.
package report

// Level is the severity carried by an event in JSON output.
type Level string

// Severity levels emitted on JSON events.
const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Style selects the color applied to an event line in text mode. It is
// independent of Level so a positive outcome can be green while still
// carrying info severity in JSON output.
type Style int

// Text-mode color styles.
const (
	StyleNone Style = iota
	StyleSuccess
	StyleWarning
	StyleError
)

// Field is one key/value pair attached to an event. Fields are ordered so
// callers control which keys appear; JSON key order itself is not
// significant to consumers.
type Field struct {
	Key   string
	Value any
}

// F constructs a Field.
func F(key string, value any) Field {
	return Field{Key: key, Value: value}
}

// Event is a single unit of command output. The text reporter renders
// Message (colored by Style); the JSON reporter renders Name plus Fields,
// adding a timestamp and Level.
type Event struct {
	Level   Level
	Style   Style
	Name    string
	Message string
	Fields  []Field
}
