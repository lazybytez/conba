// Package docker implements the runtime.Runtime interface using the
// Docker Engine API via the official Go SDK.
package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"

	"github.com/lazybytez/conba/internal/runtime"
)

// Client implements runtime.Runtime using the Docker Engine API.
type Client struct {
	docker *client.Client
}

// New creates a Docker runtime client and verifies connectivity by pinging
// the daemon. If host is empty, the client is configured from environment
// variables (DOCKER_HOST, etc.). Otherwise the given host is used directly.
func New(ctx context.Context, host string) (*Client, error) {
	opts := []client.Opt{
		client.WithHost(host),
		client.WithAPIVersionNegotiation(),
	}

	docker, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	_, err = docker.Ping(ctx)
	if err != nil {
		return nil, fmt.Errorf("ping docker daemon: %w", err)
	}

	return &Client{docker: docker}, nil
}

// ListContainers returns metadata for all currently running containers.
func (c *Client) ListContainers(ctx context.Context) ([]runtime.ContainerInfo, error) {
	containers, err := c.docker.ContainerList(ctx, container.ListOptions{
		All: false,
	})
	if err != nil {
		return nil, fmt.Errorf("list docker containers: %w", err)
	}

	infos := make([]runtime.ContainerInfo, 0, len(containers))

	for _, ctr := range containers {
		info := runtime.ContainerInfo{
			ID:     ctr.ID,
			Name:   containerName(ctr.Names),
			Labels: ctr.Labels,
			Mounts: mapMounts(ctr.Mounts),
		}

		infos = append(infos, info)
	}

	return infos, nil
}

// Close releases resources held by the Docker client.
func (c *Client) Close() error {
	err := c.docker.Close()
	if err != nil {
		return fmt.Errorf("close docker client: %w", err)
	}

	return nil
}

// containerName extracts the container name from the Docker names slice,
// stripping the leading slash that the Docker API prepends.
func containerName(names []string) string {
	if len(names) == 0 {
		return ""
	}

	return strings.TrimPrefix(names[0], "/")
}

// mapMounts converts Docker mount points to runtime.MountInfo values.
func mapMounts(mounts []container.MountPoint) []runtime.MountInfo {
	infos := make([]runtime.MountInfo, 0, len(mounts))

	for _, mount := range mounts {
		name := mount.Name
		if mount.Type == runtime.MountTypeBind {
			name = mount.Source
		}

		infos = append(infos, runtime.MountInfo{
			Type:        string(mount.Type),
			Name:        name,
			Destination: mount.Destination,
			ReadOnly:    !mount.RW,
		})
	}

	return infos
}
