package report

import "context"

type contextKey struct{}

// WithReporter returns a new context carrying the reporter.
func WithReporter(ctx context.Context, reporter Reporter) context.Context {
	return context.WithValue(ctx, contextKey{}, reporter)
}

// FromContext retrieves the reporter from ctx. It returns a no-op reporter
// (never nil) when none is set, so callers can emit unconditionally.
//
//nolint:ireturn // returns the Reporter abstraction by design.
func FromContext(ctx context.Context) Reporter {
	reporter, ok := ctx.Value(contextKey{}).(Reporter)
	if !ok {
		return Nop()
	}

	return reporter
}
