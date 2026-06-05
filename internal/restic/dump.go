package restic

import (
	"context"
	"fmt"
	"io"
)

// Dump streams the contents of filename inside snapshotID to stdout. The
// orchestrator typically pipes this output into another process (for
// example, docker exec -i feeding a database client) via io.Pipe.
// A non-zero exit status from restic is wrapped as ErrResticFailed.
func (c *Client) Dump(
	ctx context.Context,
	snapshotID string,
	filename string,
	stdout io.Writer,
) error {
	err := c.runStreaming(ctx, BuildDumpArgs(snapshotID, filename), stdout)
	if err != nil {
		return fmt.Errorf("restic dump: %w", err)
	}

	return nil
}
