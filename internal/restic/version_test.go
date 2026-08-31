package restic_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lazybytez/conba/internal/restic"
)

// probeEnvVar is the marker the environment-isolation fixture looks for. It
// must never reach the probed binary.
const probeEnvVar = "CONBA_PROBE_LEAK"

// lingerBound is the wall-clock ceiling for a probe whose descendant keeps the
// stdout pipe open longer than the probed binary itself lives.
const lingerBound = 3 * time.Second

func TestParseResticVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"standard line", "restic 0.18.1 compiled with go1.25.1 on linux/arm64\n", "0.18.1"},
		{"extra spaces", "  restic   0.17.3  compiled\n", "0.17.3"},
		{"unrecognized", "some other tool 1.2.3", ""},
		{"empty", "", ""},
		{"ansi escape in token", "restic 0.18\x1b[31m.1 compiled\n", ""},
		{"long distro-patched token", "restic " + strings.Repeat("9", 33), strings.Repeat("9", 33)},
		{"over-long token", "restic " + strings.Repeat("9", 65), ""},
		{"non-ascii token", "restic 0.18.1é\n", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.ParseResticVersion(test.in)
			if got != test.want {
				t.Errorf("ParseResticVersion(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}

// TestPlausibleVersionToken covers the predicate guarding the token read from
// the probed binary's stdout. Whitespace never survives strings.Fields, so the
// space and newline cases are only reachable at this level.
func TestPlausibleVersionToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{"normal version", "0.18.1", true},
		{"ansi escape", "0.18\x1b[31m.1", false},
		{"newline", "0.18.1\nrestic: fake", false},
		{"at the length cap", strings.Repeat("9", 64), true},
		{"over-long", strings.Repeat("9", 65), false},
		{"space", "0.18.1 fake", false},
		{"empty", "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got := restic.PlausibleVersionToken(test.token)
			if got != test.want {
				t.Errorf("PlausibleVersionToken(%q) = %v, want %v", test.token, got, test.want)
			}
		})
	}
}

// TestDetectVersion_RejectsImplausibleToken asserts a binary whose version
// line carries a token that cannot be a version is reported as unparseable,
// which routes the CLI to its unreadable path instead of rendering the token.
func TestDetectVersion_RejectsImplausibleToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
	}{
		{"ansi escape", "restic 0.18\x1b[31m.1 compiled with go1.25.1"},
		{"over-long token", "restic " + strings.Repeat("9", 65)},
		{"non-ascii token", "restic 0.18.1é"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			binary := writeProbeScript(t, "printf '%s\\n' '"+test.line+"'\n")

			_, err := restic.DetectVersion(context.Background(), binary)
			if !errors.Is(err, restic.ErrResticVersionParse) {
				t.Errorf("DetectVersion() error = %v, want ErrResticVersionParse", err)
			}
		})
	}
}

func TestVersionsCompatible(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		a, b      string
		wantMatch bool
		wantOK    bool
	}{
		{"same version", "0.18.1", "0.18.1", true, true},
		{"patch differs is compatible", "0.18.1", "0.18.5", true, true},
		{"minor differs", "0.18.1", "0.17.3", false, true},
		{"major differs", "1.0.0", "0.18.1", false, true},
		{"unparseable left", "unknown", "0.18.1", false, false},
		{"unparseable right", "0.18.1", "v0.18", false, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			match, ok := restic.VersionsCompatible(test.a, test.b)
			if match != test.wantMatch || ok != test.wantOK {
				t.Errorf("VersionsCompatible(%q, %q) = (%v, %v), want (%v, %v)",
					test.a, test.b, match, ok, test.wantMatch, test.wantOK)
			}
		})
	}
}

// TestDetectVersion exercises the real restic binary resolved from PATH. It
// skips when restic is not installed.
func TestDetectVersion(t *testing.T) {
	t.Parallel()

	version, err := restic.DetectVersion(context.Background(), "restic")
	if err != nil {
		t.Skipf("restic not available: %v", err)
	}

	if _, ok := restic.VersionsCompatible(version, version); !ok {
		t.Errorf("detected version %q does not parse", version)
	}
}

// TestDetectVersion_RunsWithEmptyEnvironment asserts the probe does not hand
// the caller's environment, which carries repository secrets, to the binary.
func TestDetectVersion_RunsWithEmptyEnvironment(t *testing.T) {
	t.Setenv(probeEnvVar, "leaked")

	script := fmt.Sprintf("echo \"restic 0.0.${%s:-clean}\"\n", probeEnvVar)

	got, err := restic.DetectVersion(context.Background(), writeProbeScript(t, script))
	if err != nil {
		t.Fatalf("DetectVersion() returned error: %v", err)
	}

	if got != "0.0.clean" {
		t.Errorf("DetectVersion() = %q, want %q: the probe inherited the environment",
			got, "0.0.clean")
	}
}

// TestDetectVersion_BoundsWaitOnLingeringDescendant asserts a descendant that
// outlives the probed binary while holding its stdout pipe cannot stretch the
// probe past its bound.
func TestDetectVersion_BoundsWaitOnLingeringDescendant(t *testing.T) {
	t.Parallel()

	binary := writeProbeScript(t, "sleep 5 &\necho \"restic 0.0.0\"\n")

	start := time.Now()

	_, err := restic.DetectVersion(context.Background(), binary)

	elapsed := time.Since(start)
	if elapsed > lingerBound {
		t.Errorf("DetectVersion() took %s, want at most %s", elapsed, lingerBound)
	}

	if !errors.Is(err, exec.ErrWaitDelay) {
		t.Errorf("DetectVersion() error = %v, want exec.ErrWaitDelay", err)
	}
}

// TestDetectVersion_ErrorChainSeparatesAbsentFromUnrunnable asserts the
// wrapping in DetectVersion keeps the chain intact, so a caller can tell a
// binary that is not there from one that is there but cannot be run.
func TestDetectVersion_ErrorChainSeparatesAbsentFromUnrunnable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		binary     string
		wantAbsent bool
	}{
		{"path to a missing file", filepath.Join(t.TempDir(), "restic"), true},
		{"unresolvable bare name", "conba-restic-does-not-exist", true},
		{"file without an execute bit", writeUnrunnableProbe(t), false},
		{"binary exiting non-zero", writeProbeScript(t, "exit 3\n"), false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := restic.DetectVersion(context.Background(), test.binary)
			if err == nil {
				t.Fatalf("DetectVersion(%q) returned no error", test.binary)
			}

			absent := errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist)
			if absent != test.wantAbsent {
				t.Errorf("DetectVersion(%q) error = %v, absent = %v, want absent = %v",
					test.binary, err, absent, test.wantAbsent)
			}
		})
	}
}

// writeProbeScript writes an executable shell script standing in for the
// restic binary and returns its path.
func writeProbeScript(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "restic")

	//nolint:gosec // the fixture must be executable for the probe to run it
	err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o700)
	if err != nil {
		t.Fatalf("write probe script to %q: %v", path, err)
	}

	return path
}

// writeUnrunnableProbe writes a file that exists where restic is expected but
// carries no execute bit, which denies exec even to root.
func writeUnrunnableProbe(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "restic")

	err := os.WriteFile(path, []byte("#!/bin/sh\necho 'restic 0.18.1'\n"), 0o600)
	if err != nil {
		t.Fatalf("write probe file to %q: %v", path, err)
	}

	return path
}
