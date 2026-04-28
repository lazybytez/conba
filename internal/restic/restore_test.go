package restic_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestRestore_Success(t *testing.T) {
	t.Parallel()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)
	ctx := context.Background()

	err := client.Init(ctx)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	dataDir := t.TempDir()
	createTestFile(t, dataDir, "hello.txt", "hello world")

	err = client.Backup(ctx, dataDir, []string{"restore-test"})
	if err != nil {
		t.Fatalf("backup: %v", err)
	}

	snapshots, err := client.Snapshots(ctx, []string{"restore-test"})
	if err != nil {
		t.Fatalf("snapshots: %v", err)
	}

	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}

	target := filepath.Join(t.TempDir(), "restore")

	err = client.Restore(ctx, snapshots[0].ID, target, false)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	// Restic restores into target preserving the source path tree.
	//nolint:gosec // test fixture path; no untrusted input
	got, err := os.ReadFile(filepath.Join(target, dataDir, "hello.txt"))
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}

	if string(got) != "hello world" {
		t.Errorf("expected restored content %q, got %q", "hello world", string(got))
	}
}

func TestRestore_NonZeroExit(t *testing.T) {
	t.Parallel()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)
	ctx := context.Background()

	err := client.Init(ctx)
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	target := filepath.Join(t.TempDir(), "restore")

	err = client.Restore(ctx, "doesnotexist123", target, false)
	if err == nil {
		t.Fatal("expected error for unknown snapshot id, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}
