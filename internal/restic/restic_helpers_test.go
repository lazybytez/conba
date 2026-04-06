package restic_test

import (
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/lazybytez/conba/internal/config"
	"github.com/lazybytez/conba/internal/restic"
	"go.uber.org/zap"
)

func fakeResticPath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine test file path")
	}

	return filepath.Join(filepath.Dir(file), "testdata", "fake_restic.sh")
}

func newHelperClient(
	t *testing.T,
	exitCode int,
	stdout string,
	stderr string,
) *restic.Client {
	t.Helper()

	cfg := config.ResticConfig{
		Binary:       fakeResticPath(t),
		Repository:   "",
		Password:     "",
		PasswordFile: "",
		ExtraArgs:    nil,
		Environment: map[string]string{
			"go_helper_exit_code": strconv.Itoa(exitCode),
			"go_helper_stdout":    stdout,
			"go_helper_stderr":    stderr,
		},
	}

	return restic.New(cfg, zap.NewNop())
}
