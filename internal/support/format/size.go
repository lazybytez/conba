// Package format provides human-readable formatting utilities.
package format

import "fmt"

const (
	kib = 1024
	mib = kib * 1024
	gib = mib * 1024
	tib = gib * 1024
)

// Bytes formats a byte count as a human-readable string using binary
// units (B, KiB, MiB, GiB, TiB).
func Bytes(bytes uint64) string {
	switch {
	case bytes >= tib:
		return fmt.Sprintf("%.2f TiB", float64(bytes)/float64(tib))
	case bytes >= gib:
		return fmt.Sprintf("%.2f GiB", float64(bytes)/float64(gib))
	case bytes >= mib:
		return fmt.Sprintf("%.2f MiB", float64(bytes)/float64(mib))
	case bytes >= kib:
		return fmt.Sprintf("%.2f KiB", float64(bytes)/float64(kib))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
