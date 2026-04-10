package restic

import (
	"context"
	"fmt"
)

// Stats returns repository statistics including total size and file count.
func (c *Client) Stats(ctx context.Context) (RepoStats, error) {
	out, err := c.run(ctx, BuildStatsArgs())
	if err != nil {
		return RepoStats{}, fmt.Errorf("restic stats: %w", err)
	}

	return ParseStats(out)
}
