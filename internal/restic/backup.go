package restic

import (
	"context"
	"fmt"
)

// Backup runs a restic backup of the given path with optional tags.
func (c *Client) Backup(ctx context.Context, path string, tags []string) error {
	_, err := c.run(ctx, BuildBackupArgs(path, tags))
	if err != nil {
		return fmt.Errorf("restic backup: %w", err)
	}

	return nil
}
