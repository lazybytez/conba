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

// binaryPath is the path to the freshly built conba binary. Populated by
// TestMain before m.Run(); a package-level var is the standard TestMain
// handoff since TestMain has no *testing.T.
//
//nolint:gochecknoglobals // TestMain pattern requires package-level state.
var binaryPath string

// requiredServices must stay in sync with test/e2e/compose.yaml.
//
//nolint:gochecknoglobals // Fixture manifest shared with helpers_test.go.
var requiredServices = []string{
	containerMySQL,
	containerApp,
	containerIgnored,
}

var errGoModNotFound = errors.New("go.mod not found")

var errServiceUnhealthy = errors.New("compose service is not healthy")

// TestMain builds the conba binary, verifies the compose fixture is healthy,
// then runs the e2e suite. Exits 2 if the fixture is not up; bringing it up
// is the caller's responsibility (typically `make go/test-e2e/up`).
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

// findModuleRoot walks upward from cwd until it finds a go.mod.
// The result is passed to `go build -C`.
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

// buildBinary compiles ./cmd/conba from moduleRoot into outPath.
// Uses `go build -C` so the call is independent of cwd.
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

// verifyFixtureHealthy returns the first service that is missing or
// whose health status is not "healthy".
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
