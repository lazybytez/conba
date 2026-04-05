// Package build provides version and build metadata for the conba CLI.
// Values are injected at compile time via -ldflags -X.
package build

import "runtime"

// Version is the release version, injected via ldflags.
// Defaults to "edge" when not set at build time.
var Version = "edge"

// CommitSHA is the git commit hash, injected via ldflags.
// Defaults to "unknown" when not set at build time.
var CommitSHA = "unknown"

// ResticVersion is the pinned restic version bundled in the container image.
// Injected via ldflags in container builds, defaults to the pinned version.
var ResticVersion = "0.18.1"

// FormatVersion returns a human-readable version string for the given
// version and commit SHA. Edge builds include the SHA, releases do not.
func FormatVersion(version, commitSHA string) string {
	if version == "edge" {
		return "edge<" + commitSHA + ">"
	}

	return version
}

// ComputeVersionString returns the version string using the injected build values.
func ComputeVersionString() string {
	return FormatVersion(Version, CommitSHA)
}

// GoVersion returns the Go runtime version used to compile the binary.
func GoVersion() string {
	return runtime.Version()
}
