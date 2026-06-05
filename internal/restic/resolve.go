package restic

import (
	"errors"
	"fmt"
	"slices"
)

// Sentinel errors for snapshot resolution failures.
var (
	// ErrSnapshotNotFound indicates no snapshot in the supplied list matches
	// the resolution criteria (required tags or explicit ID).
	ErrSnapshotNotFound = errors.New("snapshot not found")

	// ErrSnapshotTagMismatch indicates an explicitly-requested snapshot ID
	// exists but does not carry every required tag.
	ErrSnapshotTagMismatch = errors.New("snapshot tag mismatch")
)

// ResolveSnapshot picks a snapshot from the supplied list. When explicitID
// is non-empty, it returns the snapshot with that ID after verifying it
// carries every tag in requiredTags. When explicitID is empty, it returns
// the most recent snapshot (by Time) that carries every tag in requiredTags.
// An empty requiredTags slice matches every snapshot.
func ResolveSnapshot(
	snapshots []Snapshot,
	requiredTags []string,
	explicitID string,
) (Snapshot, error) {
	if explicitID != "" {
		return resolveByID(snapshots, requiredTags, explicitID)
	}

	return resolveLatestByTags(snapshots, requiredTags)
}

// resolveByID returns the snapshot whose ID matches explicitID, after
// verifying it carries every required tag.
func resolveByID(snapshots []Snapshot, requiredTags []string, explicitID string) (Snapshot, error) {
	for _, snap := range snapshots {
		if snap.ID != explicitID {
			continue
		}

		if !hasAllTags(snap, requiredTags) {
			return Snapshot{}, fmt.Errorf(
				"%w: snapshot %q does not carry all required tags %v",
				ErrSnapshotTagMismatch, explicitID, requiredTags,
			)
		}

		return snap, nil
	}

	return Snapshot{}, fmt.Errorf("%w: id %q", ErrSnapshotNotFound, explicitID)
}

// resolveLatestByTags returns the most recent snapshot (by Time) that
// carries every required tag.
func resolveLatestByTags(snapshots []Snapshot, requiredTags []string) (Snapshot, error) {
	var (
		best  Snapshot
		found bool
	)

	for _, snap := range snapshots {
		if !hasAllTags(snap, requiredTags) {
			continue
		}

		if !found || snap.Time.After(best.Time) {
			best = snap
			found = true
		}
	}

	if !found {
		return Snapshot{}, fmt.Errorf(
			"%w: no snapshot carries all required tags %v",
			ErrSnapshotNotFound, requiredTags,
		)
	}

	return best, nil
}

// hasAllTags reports whether snap carries every tag in required.
// An empty required slice trivially matches every snapshot.
func hasAllTags(snap Snapshot, required []string) bool {
	for _, want := range required {
		if !containsTag(snap.Tags, want) {
			return false
		}
	}

	return true
}

func containsTag(tags []string, want string) bool {
	return slices.Contains(tags, want)
}
