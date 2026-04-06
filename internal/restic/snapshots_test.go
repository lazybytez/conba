package restic_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestSnapshots_WithBackup(t *testing.T) {
	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)
	ctx := context.Background()

	err := client.Init(ctx)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	dataDir := t.TempDir()
	createTestFile(t, dataDir, "testfile.txt", "hello world")

	err = client.Backup(ctx, dataDir, []string{"snap-test"})
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}

	snapshots, err := client.Snapshots(ctx, []string{"snap-test"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}

	snap := snapshots[0]

	if snap.ID == "" {
		t.Error("expected non-empty snapshot ID")
	}

	if !slices.Contains(snap.Tags, "snap-test") {
		t.Errorf("expected tags to contain %q, got %v", "snap-test", snap.Tags)
	}

	if !slices.Contains(snap.Paths, dataDir) {
		t.Errorf("expected paths to contain %q, got %v", dataDir, snap.Paths)
	}
}

func TestSnapshots_Empty(t *testing.T) {
	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)
	ctx := context.Background()

	err := client.Init(ctx)
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	snapshots, err := client.Snapshots(ctx, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(snapshots) != 0 {
		t.Errorf("expected empty slice, got %d snapshots", len(snapshots))
	}
}

func TestSnapshots_Failure(t *testing.T) {
	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)
	ctx := context.Background()

	_, err := client.Snapshots(ctx, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}
