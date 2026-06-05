package restic

import (
	"context"
	"fmt"
)

// Diff compares two snapshots and returns restic's diff output as bytes.
// snapA and snapB may be full IDs, short IDs, or the literal "latest".
// Non-zero exit propagates as a wrapped error matchable via
// errors.Is(err, ErrResticFailed).
func (c *Client) Diff(ctx context.Context, snapA, snapB string) ([]byte, error) {
	out, err := c.run(ctx, BuildDiffArgs(snapA, snapB))
	if err != nil {
		return nil, fmt.Errorf("restic diff: %w", err)
	}

	return out, nil
}
