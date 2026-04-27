package restic_test

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/restic"
	"go.uber.org/zap"
)

func TestBackupFromCommand_Success(t *testing.T) {
	t.Parallel()

	client := newStreamTestClient(t)

	err := client.Init(context.Background())
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	err = client.BackupFromCommand(
		context.Background(),
		"dump.txt",
		[]string{"test-tag"},
		[]string{"/bin/sh", "-c", "echo hello"},
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestBackupFromCommand_NonZeroExit(t *testing.T) {
	t.Parallel()

	client := newStreamTestClient(t)

	err := client.Init(context.Background())
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	err = client.BackupFromCommand(
		context.Background(),
		"dump.txt",
		[]string{"test-tag"},
		[]string{"/bin/sh", "-c", "echo broken >&2; exit 1"},
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}

func TestBackupFromCommand_RepoMissing(t *testing.T) {
	t.Parallel()

	client := newStreamTestClient(t)

	err := client.BackupFromCommand(
		context.Background(),
		"dump.txt",
		[]string{"test-tag"},
		[]string{"/bin/sh", "-c", "echo hello"},
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}

// newStreamTestClient builds a restic client for stream-backup tests.
// Restic's --stdin-from-command spawns a child process and uses its
// cache directory, so HOME and PATH must be present in the env we
// pass to restic; they are not in the default test config.
func newStreamTestClient(t *testing.T) *restic.Client {
	t.Helper()

	binary, err := exec.LookPath("restic")
	if err != nil {
		t.Fatal("restic binary not found in PATH")
	}

	repoPath := filepath.Join(t.TempDir(), "repo")
	cacheDir := filepath.Join(t.TempDir(), "cache")

	cfg := config.ResticConfig{
		Binary:       binary,
		Repository:   repoPath,
		Password:     "test-password",
		PasswordFile: "",
		ExtraArgs:    nil,
		Environment: map[string]string{
			"PATH":             "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
			"RESTIC_CACHE_DIR": cacheDir,
		},
	}

	client, err := restic.New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}

	return client
}
