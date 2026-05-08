// Package backup orchestrates restic backup operations for discovered
// container-volume targets.
package backup

import (
	"github.com/lazybytez/conba/internal/discovery"
)

// Restic tag prefixes used by conba. Backup writes them; forget and
// snapshots commands read them. Defining them here keeps producers and
// consumers in lockstep — change the schema in one place.
const (
	ContainerTagPrefix = "container="
	VolumeTagPrefix    = "volume="
	HostTagPrefix      = "hostname="
)

// BuildTags returns restic tags for a backup target.
// Tags are deterministic: container, volume, hostname.
func BuildTags(target discovery.Target, hostname string) []string {
	return []string{
		ContainerTagPrefix + target.Container.Name,
		VolumeTagPrefix + target.Mount.Name,
		HostTagPrefix + hostname,
	}
}
