package docker

import "github.com/docker/docker/api/types/container"

// Exported aliases for unexported functions, used by tests in docker_test package.
var (
	ContainerName = containerName
	MapMounts     = mapMounts
	NewTailWriter = newTailWriter
	ExecExitError = execExitError

	ErrExecNonZeroExit = errExecNonZeroExit
)

// TailWriter re-exports the bounded stderr tail writer for tests.
type TailWriter = tailWriter

// MountPoint re-exports the Docker type for test convenience.
type MountPoint = container.MountPoint
