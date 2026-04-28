package restic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestDiff_Success(t *testing.T) {
	t.Parallel()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)

	err := client.Init(context.Background())
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	dataDir := t.TempDir()
	createTestFile(t, dataDir, "first.txt", "first")

	err = client.Backup(context.Background(), dataDir, []string{"first"})
	if err != nil {
		t.Fatalf("first backup failed: %v", err)
	}

	createTestFile(t, dataDir, "second.txt", "second")

	err = client.Backup(context.Background(), dataDir, []string{"second"})
	if err != nil {
		t.Fatalf("second backup failed: %v", err)
	}

	firstSnaps, err := client.Snapshots(context.Background(), []string{"first"})
	if err != nil {
		t.Fatalf("listing first snapshots failed: %v", err)
	}

	if len(firstSnaps) != 1 {
		t.Fatalf("want exactly 1 'first' snapshot, got %d", len(firstSnaps))
	}

	secondSnaps, err := client.Snapshots(context.Background(), []string{"second"})
	if err != nil {
		t.Fatalf("listing second snapshots failed: %v", err)
	}

	if len(secondSnaps) != 1 {
		t.Fatalf("want exactly 1 'second' snapshot, got %d", len(secondSnaps))
	}

	out, err := client.Diff(
		context.Background(),
		firstSnaps[0].ID,
		secondSnaps[0].ID,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(out) == 0 {
		t.Error("expected non-empty diff output")
	}
}

func TestDiff_Failure(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, "/proc/nonexistent/repo", "test-password")

	_, err := client.Diff(context.Background(), "abc", "def")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}
