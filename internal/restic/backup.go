package restic

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Backup runs a restic backup of the given path with optional tags.
//
// Backup performs a pre-flight os.Stat on the source path so missing or
// unreadable sources can be reported as ErrSourceUnreadable instead of
// surfacing as opaque restic subprocess failures. os.Stat (not os.Lstat)
// follows symlinks, mirroring the way restic itself resolves the source
// argument; this keeps the pre-flight semantics aligned with the
// subsequent restic invocation.
func (c *Client) Backup(ctx context.Context, path string, tags []string) error {
	_, statErr := os.Stat(path)
	if statErr != nil {
		switch {
		case errors.Is(statErr, fs.ErrNotExist), errors.Is(statErr, fs.ErrPermission):
			return fmt.Errorf("restic backup: %w: %w", ErrSourceUnreadable, statErr)
		default:
			return fmt.Errorf("restic backup: stat source: %w", statErr)
		}
	}

	_, err := c.run(ctx, BuildBackupArgs(path, tags))
	if err != nil {
		return fmt.Errorf("restic backup: %w", err)
	}

	return nil
}
