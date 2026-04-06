package logging

import (
	"context"

	"go.uber.org/zap"
)

type contextKey struct{}

// WithLogger returns a new context with the given logger attached.
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

// FromContext retrieves the logger from the context.
// Returns a no-op logger if none is set.
func FromContext(ctx context.Context) *zap.Logger {
	logger, ok := ctx.Value(contextKey{}).(*zap.Logger)
	if !ok {
		return zap.NewNop()
	}

	return logger
}
