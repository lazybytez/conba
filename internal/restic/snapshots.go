package restic

import (
	"context"
	"fmt"
)

// Snapshots lists restic snapshots, optionally filtered by tags.
func (c *Client) Snapshots(ctx context.Context, tags []string) ([]Snapshot, error) {
	out, err := c.run(ctx, BuildSnapshotArgs(tags))
	if err != nil {
		return nil, fmt.Errorf("restic snapshots: %w", err)
	}

	return ParseSnapshots(out)
}
