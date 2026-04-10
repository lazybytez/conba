package docker

import "github.com/docker/docker/api/types/container"

// Exported aliases for unexported functions, used by tests in docker_test package.
var (
	ContainerName = containerName
	MapMounts     = mapMounts
)

// MountPoint re-exports the Docker type for test convenience.
type MountPoint = container.MountPoint
