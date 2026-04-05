// Package discovery identifies container-volume pairs eligible for backup.
package discovery

import (
	"context"
	"fmt"

	"github.com/lazybytez/conba/internal/runtime"
)

// Target represents a single container-volume pair eligible for backup.
type Target struct {
	Container runtime.ContainerInfo
	Mount     runtime.MountInfo
}

// Discover queries the container runtime and returns a Target for each
// qualifying mount (volume or bind) across all running containers.
func Discover(ctx context.Context, rt runtime.Runtime) ([]Target, error) {
	containers, err := rt.ListContainers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	var targets []Target

	for _, container := range containers {
		for _, mount := range container.Mounts {
			if !isEligible(mount) {
				continue
			}

			targets = append(targets, Target{Container: container, Mount: mount})
		}
	}

	return targets, nil
}

func isEligible(m runtime.MountInfo) bool {
	switch m.Type {
	case "volume", "bind":
		return true
	default:
		return false
	}
}
