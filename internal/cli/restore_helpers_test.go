package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
)

// openSentinel creates and opens a sentinel file in dir so the directory is
// not empty. It is used by restore_test.go to drive the
// ErrDestinationNotEmpty branch of volume restore.
func openSentinel(dir string) (*os.File, error) {
	//nolint:gosec // test fixture path; no untrusted input
	f, err := os.Create(filepath.Join(dir, ".conba-sentinel"))
	if err != nil {
		return nil, fmt.Errorf("create sentinel: %w", err)
	}

	return f, nil
}
