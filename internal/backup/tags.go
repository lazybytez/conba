// Package backup orchestrates restic backup operations for discovered
// container-volume targets.
package backup

import (
	"github.com/lazybytez/conba/internal/discovery"
)

// Restic tag prefixes used by conba. Backup writes them; forget and
// snapshots commands read them. Defining them here keeps producers and
// consumers in lockstep: change the schema in one place.
const (
	ContainerTagPrefix = "container="
	VolumeTagPrefix    = "volume="
	HostTagPrefix      = "hostname="
	KindTagPrefix      = "kind="
)

// StreamKind is the kind tag value conba writes on stream (command-output)
// snapshots to distinguish them from volume snapshots.
const StreamKind = "stream"

// VolumeKind is the kind tag value conba writes on volume snapshots to
// distinguish them from stream snapshots.
const VolumeKind = "volume"

// BuildTags returns the forget-match IDENTITY set for a backup target:
// container, volume, hostname. It must NOT carry a kind tag. forget passes
// this set to restic as a match filter, so adding kind here would stop it
// matching volume snapshots written before kind existed, leaking them past
// retention. Use BuildVolumeTags for the write path.
func BuildTags(target discovery.Target, hostname string) []string {
	return []string{
		ContainerTagPrefix + target.Container.Name,
		VolumeTagPrefix + target.Mount.Name,
		HostTagPrefix + hostname,
	}
}

// BuildVolumeTags returns restic tags for a volume backup write: the
// identity set plus kind=volume. Symmetric to BuildStreamTags. Distinct
// from BuildTags so the forget match filter stays identity-only.
func BuildVolumeTags(target discovery.Target, hostname string) []string {
	return []string{
		ContainerTagPrefix + target.Container.Name,
		VolumeTagPrefix + target.Mount.Name,
		HostTagPrefix + hostname,
		KindTagPrefix + VolumeKind,
	}
}

// BuildStreamTags returns restic tags for a stream backup of a container's
// command output. Tags are deterministic: container, hostname, kind=stream.
func BuildStreamTags(containerName, hostname string) []string {
	return []string{
		ContainerTagPrefix + containerName,
		HostTagPrefix + hostname,
		KindTagPrefix + StreamKind,
	}
}
