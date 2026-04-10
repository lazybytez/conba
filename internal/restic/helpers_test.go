package restic_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/restic"
	"go.uber.org/zap"
)

func newTestRepo(t *testing.T) (string, string) {
	t.Helper()

	repoPath := filepath.Join(t.TempDir(), "repo")

	return repoPath, "test-password"
}

func newTestClient(t *testing.T, repoPath string, password string) *restic.Client {
	t.Helper()

	binary, err := exec.LookPath("restic")
	if err != nil {
		t.Fatal("restic binary not found in PATH")
	}

	cfg := config.ResticConfig{
		Binary:       binary,
		Repository:   repoPath,
		Password:     password,
		PasswordFile: "",
		ExtraArgs:    nil,
		Environment:  nil,
	}

	client, err := restic.New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("create test client: %v", err)
	}

	return client
}

func createTestFile(t *testing.T, dir string, name string, content string) {
	t.Helper()

	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

func createBackups(t *testing.T, client *restic.Client, dataDir string, tags []string, count int) {
	t.Helper()

	ctx := context.Background()

	for idx := range count {
		createTestFile(t, dataDir, fmt.Sprintf("file%d.txt", idx), fmt.Sprintf("v%d", idx))

		err := client.Backup(ctx, dataDir, tags)
		if err != nil {
			t.Fatalf("backup %d: %v", idx+1, err)
		}
	}
}
