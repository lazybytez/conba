package restic

import (
	"context"
	"fmt"
)

// Check verifies repository integrity. When readData is true, restic also
// reads and verifies every data blob (slow); otherwise only repository
// structure is checked. Non-zero exit propagates as a wrapped error.
func (c *Client) Check(ctx context.Context, readData bool) error {
	_, err := c.run(ctx, BuildCheckArgs(readData))
	if err != nil {
		return fmt.Errorf("restic check: %w", err)
	}

	return nil
}
