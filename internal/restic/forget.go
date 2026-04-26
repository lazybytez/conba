package restic

import (
	"context"
	"fmt"
)

// ForgetOptions controls non-policy aspects of a forget call.
type ForgetOptions struct {
	Prune  bool
	DryRun bool
}

// Forget removes old snapshots according to the given retention policy and tags.
// When opts.Prune is true the underlying restic invocation also reclaims disk
// space; when opts.DryRun is true restic reports what would be forgotten without
// applying changes.
func (c *Client) Forget(
	ctx context.Context,
	tags []string,
	policy ForgetPolicy,
	opts ForgetOptions,
) error {
	_, err := c.run(ctx, BuildForgetArgs(tags, policy, opts))
	if err != nil {
		return fmt.Errorf("restic forget: %w", err)
	}

	return nil
}
