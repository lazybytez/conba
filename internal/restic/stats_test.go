package restic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestStats_AfterBackup(t *testing.T) {
	t.Parallel()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)
	ctx := context.Background()

	err := client.Init(ctx)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	dataDir := t.TempDir()
	createTestFile(t, dataDir, "testfile.txt", "hello world")

	err = client.Backup(ctx, dataDir, []string{"stats-test"})
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	stats, err := client.Stats(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if stats.TotalSize == 0 {
		t.Error("expected TotalSize > 0")
	}

	if stats.TotalFileCount == 0 {
		t.Error("expected TotalFileCount > 0")
	}
}

func TestStats_EmptyRepo(t *testing.T) {
	t.Parallel()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)
	ctx := context.Background()

	err := client.Init(ctx)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	_, err = client.Stats(ctx)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestStats_Failure(t *testing.T) {
	t.Parallel()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)
	ctx := context.Background()

	_, err := client.Stats(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}
