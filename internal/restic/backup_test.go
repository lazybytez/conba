package restic_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
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

func TestBackup_SourceMissing(t *testing.T) {
	t.Parallel()

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)

	missingPath := filepath.Join(t.TempDir(), "does-not-exist")

	err := client.Backup(context.Background(), missingPath, []string{"test-tag"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrSourceUnreadable) {
		t.Errorf("expected error wrapping ErrSourceUnreadable, got %v", err)
	}

	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected error chain to contain fs.ErrNotExist, got %v", err)
	}
}

func TestBackup_SourcePermissionDenied(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root bypasses chmod-0 permissions; cannot reproduce permission denied")
	}

	repoPath, password := newTestRepo(t)
	client := newTestClient(t, repoPath, password)

	dir := t.TempDir()
	deniedDir := filepath.Join(dir, "denied")

	err := os.Mkdir(deniedDir, 0o700)
	if err != nil {
		t.Fatalf("create denied dir: %v", err)
	}

	err = os.Chmod(deniedDir, 0)
	if err != nil {
		t.Fatalf("chmod denied dir: %v", err)
	}

	// Restore traversal so the t.TempDir cleanup can remove deniedDir.
	// 0o700 is required because the parent's RemoveAll needs to enumerate
	// and unlink entries beneath this directory; tighter modes break cleanup.
	t.Cleanup(func() {
		//nolint:gosec // test cleanup, no sensitive data
		_ = os.Chmod(deniedDir, 0o700)
	})

	target := filepath.Join(deniedDir, "source")

	err = client.Backup(context.Background(), target, []string{"test-tag"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrSourceUnreadable) {
		t.Errorf("expected error wrapping ErrSourceUnreadable, got %v", err)
	}

	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("expected error chain to contain fs.ErrPermission, got %v", err)
	}
}
