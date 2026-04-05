package config

import "context"

type contextKey struct{}

// WithConfig returns a new context with the given config attached.
func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, contextKey{}, cfg)
}

// FromContext retrieves the config from the context.
// Returns nil if no config is set.
func FromContext(ctx context.Context) *Config {
	cfg, ok := ctx.Value(contextKey{}).(*Config)
	if !ok {
		return nil
	}

	return cfg
}
