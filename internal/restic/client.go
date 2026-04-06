package restic

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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
func New(cfg config.ResticConfig, logger *zap.Logger) *Client {
	return &Client{
		binary: cfg.Binary,
		env:    BuildEnv(cfg),
		logger: logger,
	}
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
