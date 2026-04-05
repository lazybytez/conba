package restic

import (
	"context"
	"fmt"
	"strings"
)

// Init initialises the restic repository. If the repository is already
// initialised the call is treated as a no-op and nil is returned.
func (c *Client) Init(ctx context.Context) error {
	_, err := c.run(ctx, BuildInitArgs())
	if err == nil {
		return nil
	}

	if strings.Contains(err.Error(), "already initialized") {
		return nil
	}

	return fmt.Errorf("restic init: %w", err)
}
