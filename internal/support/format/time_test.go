package format_test

import (
	"testing"
	"time"

	"github.com/lazybytez/conba/internal/support/format"
)

func TestTime_FormatsInCanonicalLayout(t *testing.T) {
	t.Parallel()

	got := format.Time(time.Date(2026, 4, 16, 18, 41, 15, 0, time.UTC))
	want := "2026-04-16 18:41:15"

	if got != want {
		t.Errorf("Time(...) = %q, want %q", got, want)
	}
}

func TestDateTime_MatchesLayoutLiteral(t *testing.T) {
	t.Parallel()

	if format.DateTime != "2006-01-02 15:04:05" {
		t.Errorf("DateTime = %q, want canonical Go reference layout", format.DateTime)
	}
}
