package restic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestBackup_Success(t *testing.T) {
	t.Parallel()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)

	err := client.Init(context.Background())
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	dataDir := t.TempDir()
	createTestFile(t, dataDir, "hello.txt", "hello world")

	err = client.Backup(context.Background(), dataDir, []string{"test-tag"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestBackup_Failure(t *testing.T) {
	t.Parallel()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)

	dataDir := t.TempDir()
	createTestFile(t, dataDir, "hello.txt", "hello world")

	err := client.Backup(context.Background(), dataDir, []string{"test-tag"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}
