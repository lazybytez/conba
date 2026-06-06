package report_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/lazybytez/conba/internal/report"
)

func TestFromContext_EmptyReturnsUsableNop(t *testing.T) {
	t.Parallel()

	reporter := report.FromContext(context.Background())
	if reporter == nil {
		t.Fatal("FromContext returned nil")
	}

	if reporter.Mode() != report.ModeText {
		t.Errorf("nop mode = %q, want text", reporter.Mode())
	}

	// Must not panic when emitting against a context-less reporter.
	reporter.Emit(report.Event{Name: "noop", Message: "ignored"})
}

func TestWithReporter_RoundTrip(t *testing.T) {
	t.Parallel()

	want := report.New(report.ModeJSON, &bytes.Buffer{}, false)
	ctx := report.WithReporter(context.Background(), want)

	if got := report.FromContext(ctx); got != want {
		t.Errorf("FromContext returned a different reporter")
	}
}
