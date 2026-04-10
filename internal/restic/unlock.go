package restic

import (
	"context"
	"fmt"
)

// Unlock removes stale locks from the restic repository.
func (c *Client) Unlock(ctx context.Context) error {
	_, err := c.run(ctx, BuildUnlockArgs())
	if err != nil {
		return fmt.Errorf("restic unlock: %w", err)
	}

	return nil
}
