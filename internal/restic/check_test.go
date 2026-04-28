package restic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestCheck_Success(t *testing.T) {
	t.Parallel()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)

	err := client.Init(context.Background())
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	err = client.Check(context.Background(), false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestCheck_ReadData_Success(t *testing.T) {
	t.Parallel()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)

	err := client.Init(context.Background())
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	err = client.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestCheck_Failure(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, "/proc/nonexistent/repo", "test-password")

	err := client.Check(context.Background(), false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}
