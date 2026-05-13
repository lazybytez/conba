package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/discovery"
	"github.com/lazybytez/conba/internal/filter"
	"github.com/lazybytez/conba/internal/forget"
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

// discoverFiltered opens a Docker connection, lists containers and
// volumes, and applies the configured discovery filter. It returns the
// included targets together with the connection's cleanup func, which
// the caller must defer. On error the docker connection is closed
// before returning.
func discoverFiltered(
	ctx context.Context,
	cfg *config.Config,
	logger *zap.Logger,
) ([]discovery.Target, func(), error) {
	runtime, cleanup, err := connectDocker(ctx, cfg, logger)
	if err != nil {
		return nil, nil, err
	}

	targets, err := discovery.Discover(ctx, runtime)
	if err != nil {
		cleanup()

		return nil, nil, fmt.Errorf("discover volumes: %w", err)
	}

	return filter.Apply(targets, cfg.Discovery).Included, cleanup, nil
}

// buildForgetOptions assembles the forget.Options literal shared by the
// run and forget commands.
func buildForgetOptions(hostname string, allHosts, dryRun, prune bool) forget.Options {
	return forget.Options{
		Hostname: hostname,
		AllHosts: allHosts,
		DryRun:   dryRun,
		Prune:    prune,
	}
}
