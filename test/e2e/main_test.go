//go:build e2e

package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// binaryPath is the absolute filesystem path to the freshly built conba
// binary used by every scenario. Populated by TestMain before m.Run().
// A package-level var is the idiomatic handoff from TestMain to tests —
// TestMain has no *testing.T to pass state through.
//
//nolint:gochecknoglobals // TestMain pattern requires package-level state.
var binaryPath string

// requiredServices is the set of compose services that must be reporting
// healthy before the e2e suite can run. Kept in sync with
// test/e2e/compose.yaml.
//
//nolint:gochecknoglobals // Fixture manifest shared with helpers_test.go.
var requiredServices = []string{
	containerMySQL,
	containerApp,
	containerIgnored,
}

// errGoModNotFound is returned when findModuleRoot cannot locate go.mod
// while walking parent directories from the current working directory.
var errGoModNotFound = errors.New("go.mod not found")

// errServiceUnhealthy is returned when a compose service is present but
// its reported docker health status is anything other than "healthy".
var errServiceUnhealthy = errors.New("compose service is not healthy")

// TestMain builds the conba binary, verifies the compose fixture is healthy,
// then runs the e2e suite. It exits 2 with a clear error if the fixture is
// not up — bringing the fixture up is the responsibility of the caller
// (typically the `make go/test-e2e/up` target).
func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
	buildDir, err := os.MkdirTemp("", "conba-e2e-bin-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: create temp build dir: %v\n", err)

		return 2
	}

	defer func() {
		_ = os.RemoveAll(buildDir)
	}()

	moduleRoot, err := findModuleRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: locate module root: %v\n", err)

		return 2
	}

	binaryPath = filepath.Join(buildDir, "conba")

	err = buildBinary(moduleRoot, binaryPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: build conba binary: %v\n", err)

		return 2
	}

	err = verifyFixtureHealthy(requiredServices)
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: compose fixture is not up or not healthy")
		fmt.Fprintf(os.Stderr, "e2e: %v\n", err)
		fmt.Fprintln(
			os.Stderr,
			"e2e: bring the fixture up with `make go/test-e2e/up` before running these tests",
		)

		return 2
	}

	return m.Run()
}

// findModuleRoot walks parent directories from the current working
// directory until it finds a go.mod file. The resulting path is used as
// the -C argument to `go build`.
func findModuleRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getwd: %w", err)
	}

	dir := cwd

	for {
		_, err := os.Stat(filepath.Join(dir, "go.mod"))
		if err == nil {
			return dir, nil
		}

		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("stat go.mod in %q: %w", dir, err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%w starting from %q", errGoModNotFound, cwd)
		}

		dir = parent
	}
}

// buildBinary compiles ./cmd/conba from moduleRoot into outPath using the
// Go 1.20+ -C flag to avoid relying on the test binary's working directory.
func buildBinary(moduleRoot, outPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	//nolint:gosec // Fixed binary ("go") and controlled arguments under test scope.
	cmd := exec.CommandContext(
		ctx,
		"go", "build", "-C", moduleRoot,
		"-buildvcs=false", "-o", outPath, "./cmd/conba",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"go build: %w: %s",
			err, strings.TrimSpace(string(out)),
		)
	}

	return nil
}

// verifyFixtureHealthy inspects each required container and returns an
// error describing the first one that is missing or not healthy.
func verifyFixtureHealthy(services []string) error {
	for _, service := range services {
		err := inspectHealth(service)
		if err != nil {
			return err
		}
	}

	return nil
}

func inspectHealth(service string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	//nolint:gosec // Fixed binary ("docker") and controlled arguments under test scope.
	cmd := exec.CommandContext(
		ctx,
		"docker", "inspect",
		"-f", "{{.State.Health.Status}}",
		service,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"inspect %q: %w: %s",
			service, err, strings.TrimSpace(string(out)),
		)
	}

	status := strings.TrimSpace(string(out))
	if status != "healthy" {
		return fmt.Errorf(
			"%w: service %q health status is %q",
			errServiceUnhealthy, service, status,
		)
	}

	return nil
}
