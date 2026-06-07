package restic

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// ErrResticVersionParse indicates restic's `version` output did not match the
// expected "restic <version> ..." shape.
var ErrResticVersionParse = errors.New("could not parse restic version")

// majorMinorParts is the number of leading dotted components compared when
// deciding restic compatibility (major and minor; patch is ignored).
const majorMinorParts = 2

// DetectVersion runs `<binary> version` and returns the restic version it
// reports (for example "0.18.1"). binary is the restic executable name or
// path; callers pass the configured value or the default "restic".
func DetectVersion(ctx context.Context, binary string) (string, error) {
	out, err := exec.CommandContext(ctx, binary, "version").Output()
	if err != nil {
		return "", fmt.Errorf("running %q version: %w", binary, err)
	}

	version := parseResticVersion(string(out))
	if version == "" {
		return "", fmt.Errorf("%w: %q", ErrResticVersionParse, strings.TrimSpace(string(out)))
	}

	return version, nil
}

// parseResticVersion extracts the version token from restic's version line,
// e.g. "restic 0.18.1 compiled with go1.25.1 on linux/arm64" -> "0.18.1".
func parseResticVersion(output string) string {
	fields := strings.Fields(output)
	if len(fields) >= majorMinorParts && strings.EqualFold(fields[0], "restic") {
		return fields[1]
	}

	return ""
}

// VersionsCompatible reports whether two restic version strings share the
// same major and minor component; patch differences are treated as
// compatible. The first result is the compatibility verdict; the second is
// false when either version cannot be parsed, in which case the caller must
// not assert (in)compatibility.
func VersionsCompatible(a, b string) (bool, bool) {
	majorMinorA, okA := majorMinor(a)
	majorMinorB, okB := majorMinor(b)

	if !okA || !okB {
		return false, false
	}

	return majorMinorA == majorMinorB, true
}

// majorMinor returns the "major.minor" prefix of a dotted version string and
// whether both components parsed as integers. "0.18.1" -> "0.18", true.
func majorMinor(version string) (string, bool) {
	parts := strings.SplitN(version, ".", majorMinorParts+1)
	if len(parts) < majorMinorParts {
		return "", false
	}

	for _, part := range parts[:majorMinorParts] {
		_, err := strconv.Atoi(part)
		if err != nil {
			return "", false
		}
	}

	return parts[0] + "." + parts[1], true
}
