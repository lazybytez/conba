package restic

import (
	"context"
	"fmt"
)

// Restore extracts the contents of snapshotID into targetPath. When dryRun
// is true, restic reports what it would restore without writing files.
// A non-zero exit status from restic is wrapped as ErrResticFailed.
func (c *Client) Restore(
	ctx context.Context,
	snapshotID string,
	targetPath string,
	dryRun bool,
) error {
	_, err := c.run(ctx, BuildRestoreArgs(snapshotID, targetPath, dryRun))
	if err != nil {
		return fmt.Errorf("restic restore: %w", err)
	}

	return nil
}
