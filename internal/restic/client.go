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

func (c *Client) run(ctx context.Context, args []string) ([]byte, error) {
	//nolint:gosec // binary path from operator config, not user input
	cmd := exec.CommandContext(ctx, c.binary, args...)
	cmd.Env = c.env

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

// runStreaming invokes restic with the given args and streams stdout
// directly into the supplied writer. Stderr is buffered and surfaced
// in the same format as run when the subprocess exits non-zero.
func (c *Client) runStreaming(ctx context.Context, args []string, stdout io.Writer) error {
	//nolint:gosec // binary path from operator config, not user input
	cmd := exec.CommandContext(ctx, c.binary, args...)
	cmd.Env = c.env
	cmd.Stdout = stdout

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			c.logger.Warn("restic stderr",
				zap.String("stderr", stderr.String()),
				zap.String("command", args[0]),
			)

			return fmt.Errorf("%w: %s exited with code %d: %s",
				ErrResticFailed, args[0], exitErr.ExitCode(), bytes.TrimSpace(stderr.Bytes()))
		}

		return fmt.Errorf("executing restic %s: %w", args[0], err)
	}

	c.logger.Debug("restic command completed", zap.String("command", args[0]))

	return nil
}
