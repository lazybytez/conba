package restic_test

import (
	"context"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/restic"
	"go.uber.org/zap"
)

func TestBackupFromStdin_Success(t *testing.T) {
	t.Parallel()

	client := newStreamTestClient(t)

	err := client.Init(context.Background())
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	err = client.BackupFromStdin(
		context.Background(),
		"dump.txt",
		[]string{"test-tag"},
		strings.NewReader("hello world"),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestBackupFromStdin_ReaderConsumed(t *testing.T) {
	t.Parallel()

	client := newStreamTestClient(t)

	err := client.Init(context.Background())
	if err != nil {
		t.Fatalf("init repo: %v", err)
	}

	reader := &trackingReader{Reader: strings.NewReader("piped payload"), bytesRead: 0}

	err = client.BackupFromStdin(
		context.Background(),
		"dump.txt",
		[]string{"test-tag"},
		reader,
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if reader.bytesRead == 0 {
		t.Error("expected restic to consume the provided reader, but no bytes were read")
	}
}

func TestBackupFromStdin_RepoMissing(t *testing.T) {
	t.Parallel()

	client := newStreamTestClient(t)

	err := client.BackupFromStdin(
		context.Background(),
		"dump.txt",
		[]string{"test-tag"},
		strings.NewReader("hello world"),
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, restic.ErrResticFailed) {
		t.Errorf("expected error wrapping ErrResticFailed, got %v", err)
	}
}

// trackingReader records how many bytes were read so a test can assert
// that the reader was wired to the restic subprocess stdin and consumed.
type trackingReader struct {
	io.Reader

	bytesRead int
}

func (r *trackingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.bytesRead += n

	return n, err //nolint:wrapcheck // passthrough reader for test instrumentation
}

// newStreamTestClient builds a restic client for stream-backup tests.
// Restic uses a cache directory, so PATH and RESTIC_CACHE_DIR must be
// present in the env we pass to restic; they are not in the default
// test config.
func newStreamTestClient(t *testing.T) *restic.Client {
	t.Helper()

	binary, err := exec.LookPath("restic")
	if err != nil {
		t.Skipf("restic binary not found in PATH, this test requires it: %v", err)
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
