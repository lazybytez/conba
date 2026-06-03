package restic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"

	"github.com/lazybytez/conba/internal/config"
	"go.uber.org/zap"
)

// ErrResticFailed indicates a restic subprocess exited with a non-zero status.
var ErrResticFailed = errors.New("restic command failed")

// Client wraps restic CLI invocations as subprocess calls.
type Client struct {
	binary string
	env    []string
	logger *zap.Logger
}

// New creates a restic client from the given configuration and logger.
// It returns an error if the configuration is missing required fields.
func New(cfg config.ResticConfig, logger *zap.Logger) (*Client, error) {
	err := cfg.Validate()
	if err != nil {
		return nil, fmt.Errorf("invalid restic config: %w", err)
	}

	return &Client{
		binary: cfg.Binary,
		env:    BuildEnv(cfg),
		logger: logger,
	}, nil
}

// run executes restic with the given argv and no stdin, returning its stdout.
func (c *Client) run(ctx context.Context, args []string) ([]byte, error) {
	return c.runWithStdin(ctx, args, nil)
}

// runWithStdin runs restic with the given argv, attaching stdin (nil for none)
// as the subprocess standard input, and returns its stdout. A non-zero exit is
// classified as ErrResticFailed, with restic's stderr logged and included in
// the error.
func (c *Client) runWithStdin(ctx context.Context, args []string, stdin io.Reader) ([]byte, error) {
	//nolint:gosec // binary path from operator config, not user input
	cmd := exec.CommandContext(ctx, c.binary, args...)
	cmd.Env = c.env
	cmd.Stdin = stdin

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := exitErr.Stderr
			c.logger.Warn("restic stderr",
				zap.String("stderr", string(stderr)),
				zap.String("command", args[0]),
			)

			return nil, fmt.Errorf("%w: %s exited with code %d: %s",
				ErrResticFailed, args[0], exitErr.ExitCode(), bytes.TrimSpace(stderr))
		}

		return nil, fmt.Errorf("executing restic %s: %w", args[0], err)
	}

	c.logger.Debug("restic command completed", zap.String("command", args[0]))

	return out, nil
}
