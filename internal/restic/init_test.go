package restic_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lazybytez/conba/internal/restic"
)

func TestInit_Success(t *testing.T) {
	t.Parallel()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)

	err := client.Init(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	configPath := filepath.Join(repoPath, "config")

	_, statErr := os.Stat(configPath)
	if statErr != nil {
		t.Fatalf("expected repo config file at %s, got error: %v", configPath, statErr)
	}
}

func TestInit_AlreadyInitialized(t *testing.T) {
	t.Parallel()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)

	err := client.Init(context.Background())
	if err != nil {
		t.Fatalf("first init failed: %v", err)
	}

	err = client.Init(context.Background())
	if err != nil {
		t.Fatalf("second init should succeed (idempotent), got %v", err)
	}
}

func TestInit_Failure(t *testing.T) {
	t.Parallel()

	client := newTestClient(t, "/proc/nonexistent/repo", "test-password")

	err := client.Init(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}
