//go:build e2e

package e2e_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// envConbaBinary names the env var that points at the pre-built conba
// binary. `make e2e` sets it to /app/bin/conba inside the runner.
const envConbaBinary = "CONBA_BINARY"

// binaryPath is the path to the pre-built conba binary. Populated by
// TestMain from envConbaBinary before m.Run(); a package-level var is
// the standard TestMain handoff since TestMain has no *testing.T.
var binaryPath string

// requiredServices must stay in sync with test/e2e/compose.yaml.
var requiredServices = []string{
	containerMySQL,
	containerApp,
	containerIgnored,
	containerBindExcluded,
}

var errServiceUnhealthy = errors.New("compose service is not healthy")

// TestMain verifies the pre-built conba binary and compose fixture are
// available, then runs the e2e suite. Exits 2 if either is missing.
// Building the binary and bringing the fixture up is the caller's
// responsibility (typically `make e2e`).
func TestMain(m *testing.M) {
	os.Exit(runMain(m))
}

func runMain(m *testing.M) int {
	path := os.Getenv(envConbaBinary)
	if path == "" {
		fmt.Fprintf(
			os.Stderr,
			"e2e: %s env var is not set; run `make e2e` or set it explicitly\n",
			envConbaBinary,
		)

		return 2
	}

	_, err := os.Stat(path)
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"e2e: %s=%q is not accessible: %v\n",
			envConbaBinary, path, err,
		)

		return 2
	}

	binaryPath = path

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
