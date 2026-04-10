package restic_test

import (
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

	return restic.New(cfg, zap.NewNop())
}

func createTestFile(t *testing.T, dir string, name string, content string) {
	t.Helper()

	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}
