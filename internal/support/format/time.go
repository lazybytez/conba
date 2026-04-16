package format

import "time"

// DateTime is the canonical human-readable datetime layout used in CLI output.
const DateTime = "2006-01-02 15:04:05"

// Time formats t in the canonical conba datetime layout.
func Time(t time.Time) string {
	return t.Format(DateTime)
}
