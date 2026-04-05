package logging_test

import (
	"context"
	"testing"

	"github.com/lazybytez/conba/internal/logging"
	"go.uber.org/zap"
)

func TestWithLoggerAndFromContext(t *testing.T) {
	t.Parallel()

	logger := zap.NewNop()
	ctx := logging.WithLogger(context.Background(), logger)

	got := logging.FromContext(ctx)
	if got != logger {
		t.Error("FromContext did not return the logger set by WithLogger")
	}
}

func TestFromContext_ReturnsNopWhenMissing(t *testing.T) {
	t.Parallel()

	got := logging.FromContext(context.Background())
	if got == nil {
		t.Fatal("FromContext returned nil, want nop logger")
	}
}
