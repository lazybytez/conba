package restic

import (
	"context"
	"errors"
	"fmt"
)

// Init initialises the restic repository. If the repository is already
// initialised the call is treated as a no-op and nil is returned.
func (c *Client) Init(ctx context.Context) error {
	_, err := c.run(ctx, BuildInitArgs())
	if err == nil {
		return nil
	}

	if errors.Is(ClassifyError(err), ErrRepoAlreadyInitialized) {
		return nil
	}

	return fmt.Errorf("restic init: %w", err)
}
