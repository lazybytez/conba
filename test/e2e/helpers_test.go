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

// Fixture constants. Must stay in sync with test/e2e/compose.yaml and
// test/e2e/compose/mysql/*.sql.
const (
	containerMySQL        = "conba-e2e-mysql"
	containerApp          = "conba-e2e-app"
	containerIgnored      = "conba-e2e-ignored"
	containerBindExcluded = "conba-e2e-bind-excluded"

	mysqlRootUser     = "root"
	mysqlRootPassword = "conba-e2e"
	mysqlDatabase     = "conba_e2e"

	resetSQLRelPath = "compose/mysql/reset.sql"

	defaultResticPassword = "e2e-pass"
	conbaConfigFilename   = "conba.yaml"

	e2eComposeFile = "compose.yaml"

	conbaCommandTimeout  = 30 * time.Second
	dockerCommandTimeout = 15 * time.Second
)

// defaultIncludeNamePatterns pins container discovery to the e2e fixture
// so tests never accidentally target other containers on the host.
var defaultIncludeNamePatterns = []string{"^conba-e2e-"}

// runConfig configures a single invocation of the conba binary.
type runConfig struct {
	// Dir is typically a t.TempDir() containing conba.yaml.
	Dir   string
	Stdin io.Reader
	// Env, when non-nil, replaces the inherited os.Environ() completely.
	Env []string
}

// runResult holds the outcome of a conba invocation. Err is non-nil only
// for process start/wait failures; non-zero exits surface via ExitCode.
type runResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

// runConba executes the pre-built conba binary with the supplied args.
// Non-zero exit codes are reported via ExitCode and do not set Err.
func runConba(t *testing.T, cfg runConfig, args ...string) runResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), conbaCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Dir = cfg.Dir
	cmd.Stdin = cfg.Stdin
	cmd.Env = resolveEnv(cfg.Env)

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

// resolveEnv returns env verbatim when the caller supplied one; otherwise
// it inherits the current process environment.
func resolveEnv(env []string) []string {
	if env != nil {
		return env
	}

	return os.Environ()
}

// configOpts is the subset of conba.yaml fields scenarios override.
// Zero values fall back to defaults applied in writeConfig.
type configOpts struct {
	ResticRepoPath      string
	ResticPassword      string
	IncludeNames        []string
	IncludeNamePatterns []string
	ExcludeNames        []string
	Retention           config.RetentionConfig
}

// configTemplate is the minimal YAML accepted by config.Load. Field names
// must match the `mapstructure` tags on the Config struct.
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
{{- if or .Retention.KeepDaily .Retention.KeepWeekly .Retention.KeepMonthly .Retention.KeepYearly }}
retention:
{{- if .Retention.KeepDaily }}
  keep_daily: {{ .Retention.KeepDaily }}
{{- end }}
{{- if .Retention.KeepWeekly }}
  keep_weekly: {{ .Retention.KeepWeekly }}
{{- end }}
{{- if .Retention.KeepMonthly }}
  keep_monthly: {{ .Retention.KeepMonthly }}
{{- end }}
{{- if .Retention.KeepYearly }}
  keep_yearly: {{ .Retention.KeepYearly }}
{{- end }}
{{- end }}
`

// writeConfig renders a conba.yaml into dir using opts, applies defaults
// for zero fields, and verifies the result parses via the real loader.
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

// verifyConfigLoads parses the generated YAML through the real loader
// to catch template-vs-struct drift.
func verifyConfigLoads(t *testing.T, path string) {
	t.Helper()

	_, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load(%q): %v", path, err)
	}
}

// resetFixture returns the mutable fixture containers to a known-good
// state. Idempotent.
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

	composeExec(t, containerMySQL, bytes.NewReader(resetSQL),
		"mysql",
		"-u"+mysqlRootUser,
		"-p"+mysqlRootPassword,
		mysqlDatabase,
	)
}

func resetApp(t *testing.T) {
	t.Helper()

	composeExec(t, containerApp, nil,
		"sh", "-c",
		"rm -rf /data/* && mkdir -p /data/configs "+
			"&& echo hello > /data/hello.txt "+
			"&& echo v1 > /data/configs/version.txt",
	)
}

func resetIgnored(t *testing.T) {
	t.Helper()

	composeExec(t, containerIgnored, nil,
		"sh", "-c",
		"rm -rf /data/* && echo ignored > /data/should-not-be-backed-up.txt",
	)
}

// composeExec runs cmdArgs inside the named compose service via
// `docker compose exec -T`. Fails the test on non-zero exit.
func composeExec(t *testing.T, service string, stdin io.Reader, cmdArgs ...string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()

	argv := append(
		[]string{"compose", "-f", e2eComposeFile, "exec", "-T", service},
		cmdArgs...,
	)

	cmd := exec.CommandContext(ctx, "docker", argv...)
	cmd.Stdin = stdin

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"docker compose exec %s %v: %v: %s",
			service, cmdArgs, err, strings.TrimSpace(string(out)),
		)
	}
}

// readFile reads the file at path and returns its bytes. path is expected
// to live under t.TempDir(), so the gosec G304 warning is benign.
func readFile(t *testing.T, path string) []byte {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}

	return b
}

// requireStdoutContains fails the test if result.Stdout does not contain want.
func requireStdoutContains(t *testing.T, result runResult, want string) {
	t.Helper()

	if !strings.Contains(result.Stdout, want) {
		t.Fatalf("stdout does not contain %q; stdout=%q", want, result.Stdout)
	}
}

// requireSuccess fails the test unless the command ran cleanly: no start
// error and exit code 0.
func requireSuccess(t *testing.T, result runResult, cmd string) {
	t.Helper()

	if result.Err != nil {
		t.Fatalf("runConba %s: unexpected start error: %v", cmd, result.Err)
	}

	if result.ExitCode != 0 {
		t.Fatalf(
			"%s exited with %d, want 0; stderr=%q stdout=%q",
			cmd, result.ExitCode, result.Stderr, result.Stdout,
		)
	}
}

// ResticSnapshot mirrors the subset of `restic snapshots --json` fields
// consumed by the suite.
type ResticSnapshot struct {
	ShortID  string   `json:"short_id"`
	ID       string   `json:"id"`
	Paths    []string `json:"paths"`
	Tags     []string `json:"tags"`
	Hostname string   `json:"hostname"`
	Time     string   `json:"time"`
}

// resticSnapshots invokes `restic snapshots --json` against repoPath using
// defaultResticPassword. Skips the test if restic is not on PATH. An
// uninitialised repo yields a nil slice with no error.
func resticSnapshots(t *testing.T, repoPath string) []ResticSnapshot {
	t.Helper()

	stdout, stderr, err := runRestic(t, repoPath, "snapshots", "--json")
	if err != nil {
		if isResticMissingRepo(stderr) {
			return nil
		}

		t.Fatalf("restic snapshots: %v: %s", err, strings.TrimSpace(stderr))
	}

	raw := bytes.TrimSpace([]byte(stdout))
	if len(raw) == 0 {
		return nil
	}

	var snapshots []ResticSnapshot

	err = json.Unmarshal(raw, &snapshots)
	if err != nil {
		t.Fatalf("parse restic snapshots JSON: %v: %s", err, string(raw))
	}

	return snapshots
}

// resticDiff runs `restic diff a b` and returns the combined output
// trimmed of trailing whitespace. Diff failures fail the test.
func resticDiff(t *testing.T, repoPath, snapA, snapB string) string {
	t.Helper()

	stdout, stderr, err := runRestic(t, repoPath, "diff", snapA, snapB)
	if err != nil {
		t.Fatalf(
			"restic diff %s %s: %v: %s",
			snapA, snapB, err, strings.TrimSpace(stderr),
		)
	}

	return strings.TrimRight(stdout+stderr, " \t\r\n")
}

// runRestic runs the restic binary against repoPath with defaultResticPassword,
// returning stdout, stderr, and the process exit error (if any).
// Skips the test if restic is not on PATH.
func runRestic(t *testing.T, repoPath string, args ...string) (string, string, error) {
	t.Helper()

	_, err := exec.LookPath("restic")
	if err != nil {
		t.Skipf("restic binary not found in PATH, e2e harness requires it: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()

	argv := append([]string{"-r", repoPath}, args...)
	cmd := exec.CommandContext(ctx, "restic", argv...)

	cmd.Env = append(os.Environ(), "RESTIC_PASSWORD="+defaultResticPassword)

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	return stdout.String(), stderr.String(), runErr
}

// backupAsHost writes a snapshot to repoPath via direct restic invocation
// using --host hostname plus the supplied tags. sourcePath is backed up
// as-is. Used to seed foreign-host snapshots that the conba forget loop
// must respect (or affect, with --all-hosts) for host-scoping tests.
func backupAsHost(t *testing.T, repoPath, hostname, sourcePath string, tags []string) {
	t.Helper()

	args := make([]string, 0, 3+2*len(tags)+1)
	args = append(args, "backup", "--host", hostname)

	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}

	args = append(args, sourcePath)

	_, stderr, err := runRestic(t, repoPath, args...)
	if err != nil {
		t.Fatalf("restic backup --host %s %s: %v: %s",
			hostname, sourcePath, err, strings.TrimSpace(stderr))
	}
}

// isResticMissingRepo reports whether restic's stderr indicates the repo
// has not been initialised yet.
func isResticMissingRepo(stderr string) bool {
	return strings.Contains(stderr, "unable to open") ||
		strings.Contains(stderr, "does not exist") ||
		strings.Contains(stderr, "Is there a repository at")
}

// TestHarnessSelfCheck asserts the harness can invoke the built conba
// binary and capture its output. If this fails the rest of the suite
// cannot run.
func TestHarnessSelfCheck(t *testing.T) {
	result := runConba(
		t,
		runConfig{Dir: t.TempDir(), Stdin: nil, Env: nil},
		"--help",
	)

	requireSuccess(t, result, "conba --help")
	requireStdoutContains(t, result, "backup")
}
