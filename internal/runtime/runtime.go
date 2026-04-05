// Package runtime defines the interface and types for container runtime
// operations used by conba to discover and inspect running containers.
package runtime

import "context"

// ContainerInfo holds metadata about a running container.
type ContainerInfo struct {
	ID     string
	Name   string
	Labels map[string]string
	Mounts []MountInfo
}

// MountInfo describes a single mount point on a container.
type MountInfo struct {
	Type        string // "volume", "bind", "tmpfs"
	Name        string // volume name or bind source path
	Destination string // container mount path
	ReadOnly    bool
}

// Runtime abstracts container runtime operations.
type Runtime interface {
	// ListContainers returns metadata for all currently running containers.
	ListContainers(ctx context.Context) ([]ContainerInfo, error)

	// Close releases any resources held by the runtime client.
	Close() error
}
