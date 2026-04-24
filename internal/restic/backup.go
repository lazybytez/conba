package restic

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// Backup runs a restic backup of the given path with optional tags.
// It pre-flights the source path so missing or unreadable sources are
// reported as ErrSourceUnreadable instead of surfacing as opaque
// restic subprocess failures.
func (c *Client) Backup(ctx context.Context, path string, tags []string) error {
	err := checkBackupSource(path)
	if err != nil {
		return err
	}

	_, err = c.run(ctx, BuildBackupArgs(path, tags))
	if err != nil {
		return fmt.Errorf("restic backup: %w", err)
	}

	return nil
}

// checkBackupSource pre-flights the backup source path. It classifies
// fs.ErrNotExist and fs.ErrPermission as ErrSourceUnreadable so the
// caller can skip the target rather than treat it as a hard failure.
// os.Stat (not os.Lstat) follows symlinks, matching the way restic
// itself resolves the source argument.
func checkBackupSource(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}

	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("restic backup: %w: %w", ErrSourceUnreadable, err)
	}

	return fmt.Errorf("restic backup: stat source: %w", err)
}
