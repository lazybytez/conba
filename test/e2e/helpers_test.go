//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/lazybytez/conba/internal/config"
)

// Fixture constants — kept in one place so scenario tests never hardcode
// service names or credentials. Must stay in sync with
// test/e2e/compose.yaml and test/e2e/compose/mysql/*.sql.
const (
	containerMySQL   = "conba-e2e-mysql"
	containerApp     = "conba-e2e-app"
	containerIgnored = "conba-e2e-ignored"

	mysqlRootUser     = "root"
	mysqlRootPassword = "conba-e2e"
	mysqlDatabase     = "conba_e2e"

	resetSQLRelPath = "compose/mysql/reset.sql"

	defaultResticPassword = "e2e-pass"
	conbaConfigFilename   = "conba.yaml"

	conbaCommandTimeout  = 30 * time.Second
	dockerCommandTimeout = 15 * time.Second
)

// defaultIncludeNamePatterns pins container discovery to the e2e fixture
// so tests never accidentally target other containers on the host.
//
//nolint:gochecknoglobals // Shared default consumed by scenarios.
var defaultIncludeNamePatterns = []string{"^conba-e2e-"}

// runConfig configures a single invocation of the conba binary.
type runConfig struct {
	// Dir becomes cmd.Dir — typically a t.TempDir() containing conba.yaml.
	Dir string
	// Stdin is optional; nil means no stdin.
	Stdin io.Reader
	// Env, when non-nil, replaces the inherited os.Environ() completely.
	// Leave nil to inherit the current process environment.
	Env []string
}

// runResult holds the outcome of a conba invocation.
type runResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	// Err is non-nil only for process start/wait failures that are NOT a
	// non-zero exit. Callers inspect ExitCode to branch on tool behaviour.
	Err error
}

// runConba executes the pre-built conba binary with the supplied args.
// The call is bounded by a 30-second timeout. Non-zero exit codes are
// reported via ExitCode — they do NOT set Err.
func runConba(t *testing.T, cfg runConfig, args ...string) runResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), conbaCommandTimeout)
	defer cancel()

	//nolint:gosec // binaryPath is built by TestMain; args are test-controlled.
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = cfg.Dir
	cmd.Stdin = cfg.Stdin

	if cfg.Env != nil {
		cmd.Env = cfg.Env
	} else {
		cmd.Env = os.Environ()
	}

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := runResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
		Err:      nil,
	}

	if err == nil {
		return result
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()

		return result
	}

	result.Err = fmt.Errorf("running conba: %w", err)

	return result
}

// configOpts captures the knobs the e2e scenarios need to toggle in the
// generated conba.yaml. Zero values fall back to sensible defaults.
type configOpts struct {
	ResticRepoPath      string
	ResticPassword      string
	IncludeNames        []string
	IncludeNamePatterns []string
	ExcludeNames        []string
}

// configTemplate is the minimal YAML structure accepted by
// internal/config.Load(). Field names MUST match the `mapstructure` tags
// on the Config struct.
const configTemplate = `logging:
  level: info
  format: human
runtime:
  type: docker
  docker:
    host: unix:///var/run/docker.sock
discovery:
  opt_in_only: false
  include:
    names:
{{- range .IncludeNames }}
      - {{ . }}
{{- end }}
    name_patterns:
{{- range .IncludeNamePatterns }}
      - {{ printf "%q" . }}
{{- end }}
  exclude:
    names:
{{- range .ExcludeNames }}
      - {{ . }}
{{- end }}
restic:
  repository: {{ printf "%q" .ResticRepoPath }}
  password: {{ printf "%q" .ResticPassword }}
`

// writeConfig renders a conba.yaml into dir using opts. Defaults are
// applied for missing fields. The generated YAML is parsed through the
// real config loader as a sanity check that template output matches the
// Config struct field set.
func writeConfig(t *testing.T, dir string, opts configOpts) {
	t.Helper()

	if opts.ResticPassword == "" {
		opts.ResticPassword = defaultResticPassword
	}

	if opts.IncludeNamePatterns == nil {
		opts.IncludeNamePatterns = defaultIncludeNamePatterns
	}

	tpl, err := template.New("conba.yaml").Parse(configTemplate)
	if err != nil {
		t.Fatalf("parse config template: %v", err)
	}

	var buf bytes.Buffer

	err = tpl.Execute(&buf, opts)
	if err != nil {
		t.Fatalf("render config template: %v", err)
	}

	path := filepath.Join(dir, conbaConfigFilename)

	err = os.WriteFile(path, buf.Bytes(), 0o600)
	if err != nil {
		t.Fatalf("write config to %q: %v", path, err)
	}

	verifyConfigLoads(t, path)
}

// verifyConfigLoads parses the generated YAML through the real conba
// config loader as a sanity check that the template output matches the
// Config struct field set.
func verifyConfigLoads(t *testing.T, path string) {
	t.Helper()

	_, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load(%q): %v", path, err)
	}
}

// resetFixture returns all three fixture containers to a known-good
// state. Safe to call repeatedly — each step is idempotent.
func resetFixture(t *testing.T) {
	t.Helper()

	resetMySQL(t)
	resetApp(t)
	resetIgnored(t)
}

func resetMySQL(t *testing.T) {
	t.Helper()

	resetSQL, err := os.ReadFile(resetSQLRelPath)
	if err != nil {
		t.Fatalf("read %q: %v", resetSQLRelPath, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(
		ctx,
		"docker", "exec", "-i", containerMySQL,
		"mysql",
		"-u"+mysqlRootUser,
		"-p"+mysqlRootPassword,
		mysqlDatabase,
	)
	cmd.Stdin = bytes.NewReader(resetSQL)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"reset mysql: %v: %s",
			err, strings.TrimSpace(string(out)),
		)
	}
}

func resetApp(t *testing.T) {
	t.Helper()

	dockerExec(
		t,
		containerApp,
		"sh", "-c",
		"rm -rf /data/* && mkdir -p /data/configs "+
			"&& echo hello > /data/hello.txt "+
			"&& echo v1 > /data/configs/version.txt",
	)
}

func resetIgnored(t *testing.T) {
	t.Helper()

	dockerExec(
		t,
		containerIgnored,
		"sh", "-c",
		"rm -rf /data/* && echo ignored > /data/should-not-be-backed-up.txt",
	)
}

// dockerExec runs `docker exec <container> <args...>` with a bounded
// timeout and fails the test on non-zero exit.
func dockerExec(t *testing.T, container string, args ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()

	argv := append([]string{"exec", container}, args...)

	//nolint:gosec // Fixed "docker" binary and controlled args under test scope.
	cmd := exec.CommandContext(ctx, "docker", argv...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"docker exec %s %v: %v: %s",
			container, args, err, strings.TrimSpace(string(out)),
		)
	}
}

// readFile reads the file at path and returns its bytes. The path is
// expected to live under a t.TempDir() managed by the test, so the
// `gosec` G304 warning about variable-path reads is benign.
func readFile(t *testing.T, path string) []byte {
	t.Helper()

	//nolint:gosec // Path is under t.TempDir(), controlled by the test.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}

	return b
}

// requireStdoutContains fails the test if result.Stdout does not contain
// want. Keeps scenario bodies tight.
func requireStdoutContains(t *testing.T, result runResult, want string) {
	t.Helper()

	if !strings.Contains(result.Stdout, want) {
		t.Fatalf("stdout does not contain %q; stdout=%q", want, result.Stdout)
	}
}

// requireExit fails the test unless result's exit code equals want and no
// start error is set. Collapses a repeated 8-line assertion block.
//
//nolint:unparam // non-zero exit cases are expected in later scenarios.
func requireExit(t *testing.T, result runResult, cmd string, want int) {
	t.Helper()

	if result.Err != nil {
		t.Fatalf("runConba %s: unexpected start error: %v", cmd, result.Err)
	}

	if result.ExitCode != want {
		t.Fatalf(
			"%s exited with %d, want %d; stderr=%q stdout=%q",
			cmd, result.ExitCode, want, result.Stderr, result.Stdout,
		)
	}
}

// ResticSnapshot mirrors the subset of fields we parse from
// `restic snapshots --json`. Field names match the restic JSON shape.
type ResticSnapshot struct {
	ShortID  string   `json:"short_id"`
	ID       string   `json:"id"`
	Paths    []string `json:"paths"`
	Tags     []string `json:"tags"`
	Hostname string   `json:"hostname"`
	Time     string   `json:"time"`
}

// resticSnapshots invokes `restic snapshots --json` directly against
// repoPath. If the restic binary is not on PATH the test is skipped. An
// uninitialised repository yields a nil slice with no error, letting the
// caller decide whether that matters.
//
//nolint:unparam // password is parameterised to mirror resticDiff and future non-default scenarios.
func resticSnapshots(t *testing.T, repoPath, password string) []ResticSnapshot {
	t.Helper()

	_, err := exec.LookPath("restic")
	if err != nil {
		t.Skipf("restic binary not found in PATH — e2e harness requires it: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()

	//nolint:gosec // Fixed "restic" binary and controlled args under test scope.
	cmd := exec.CommandContext(ctx, "restic", "-r", repoPath, "snapshots", "--json")

	env := append(os.Environ(), "RESTIC_PASSWORD="+password)
	cmd.Env = env

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		// Uninitialised repo: restic errors out. Treat as "no snapshots"
		// so callers can distinguish empty vs uninitialised via repo path
		// existence if they need to.
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "unable to open") ||
			strings.Contains(stderrStr, "does not exist") ||
			strings.Contains(stderrStr, "Is there a repository at") {
			return nil
		}

		t.Fatalf(
			"restic snapshots: %v: %s",
			err, strings.TrimSpace(stderrStr),
		)
	}

	raw := bytes.TrimSpace(stdout.Bytes())
	if len(raw) == 0 {
		return nil
	}

	var snapshots []ResticSnapshot

	err = json.Unmarshal(raw, &snapshots)
	if err != nil {
		t.Fatalf(
			"parse restic snapshots JSON: %v: %s",
			err, string(raw),
		)
	}

	return snapshots
}

// resticDiff runs `restic diff a b` and returns the combined stdout +
// stderr output trimmed of surrounding whitespace. Both snapshots must
// exist — diff failures fail the test.
func resticDiff(t *testing.T, repoPath, password, snapA, snapB string) string {
	t.Helper()

	_, err := exec.LookPath("restic")
	if err != nil {
		t.Skipf("restic binary not found in PATH — e2e harness requires it: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()

	//nolint:gosec // Fixed "restic" binary and controlled args under test scope.
	cmd := exec.CommandContext(ctx, "restic", "-r", repoPath, "diff", snapA, snapB)

	env := append(os.Environ(), "RESTIC_PASSWORD="+password)
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"restic diff %s %s: %v: %s",
			snapA, snapB, err, strings.TrimSpace(string(out)),
		)
	}

	return strings.TrimRight(string(out), " \t\r\n")
}

// TestHarnessSelfCheck verifies that the e2e harness can invoke the built
// conba binary and capture its output. This is the bootstrap test: if it
// fails, the rest of the e2e suite cannot run.
//
//nolint:paralleltest // Suite runs with -p 1; t.Parallel() is forbidden.
func TestHarnessSelfCheck(t *testing.T) {
	result := runConba(
		t,
		runConfig{Dir: t.TempDir(), Stdin: nil, Env: nil},
		"--help",
	)

	if result.Err != nil {
		t.Fatalf("runConba returned unexpected start error: %v", result.Err)
	}

	if result.ExitCode != 0 {
		t.Fatalf(
			"conba --help exited with %d, want 0; stderr=%q",
			result.ExitCode, result.Stderr,
		)
	}

	if !strings.Contains(result.Stdout, "backup") {
		t.Fatalf(
			"conba --help stdout does not contain 'backup'; stdout=%q",
			result.Stdout,
		)
	}
}
