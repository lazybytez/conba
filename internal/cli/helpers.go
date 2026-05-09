package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/runtime/docker"
	"go.uber.org/zap"
)

var errMissingConfig = errors.New("config not available in context")

// connectDocker dials the configured Docker host and returns the client
// alongside a cleanup func suitable for `defer`. Cleanup logs (does not
// return) close errors so the caller's first error is preserved.
func connectDocker(
	ctx context.Context,
	cfg *config.Config,
	logger *zap.Logger,
) (*docker.Client, func(), error) {
	logger.Debug("connecting to docker",
		zap.String("host", cfg.Runtime.Docker.Host))

	client, err := docker.New(ctx, cfg.Runtime.Docker.Host)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to docker: %w", err)
	}

	cleanup := func() {
		closeErr := client.Close()
		if closeErr != nil {
			logger.Warn("failed to close docker client",
				zap.Error(closeErr))
		}
	}

	return client, cleanup, nil
}
