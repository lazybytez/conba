// Package backup orchestrates restic backup operations for discovered
// container-volume targets.
package backup

import (
	"github.com/lazybytez/conba/internal/discovery"
)

// BuildTags returns restic tags for a backup target.
// Tags are deterministic: container, volume, hostname.
func BuildTags(target discovery.Target, hostname string) []string {
	return []string{
		"container=" + target.Container.Name,
		"volume=" + target.Mount.Name,
		"hostname=" + hostname,
	}
}
