package restic

import (
	"context"
	"fmt"
)

// Forget removes old snapshots according to the given retention policy and tags.
func (c *Client) Forget(ctx context.Context, tags []string, policy ForgetPolicy) error {
	_, err := c.run(ctx, BuildForgetArgs(tags, policy))
	if err != nil {
		return fmt.Errorf("restic forget: %w", err)
	}

	return nil
}
