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

// ComputeVersionString returns a human-readable version string.
// For edge builds it includes the commit SHA, for releases it returns the version as-is.
func ComputeVersionString() string {
	if Version == "edge" {
		return "edge<" + CommitSHA + ">"
	}

	return Version
}

// GoVersion returns the Go runtime version used to compile the binary.
func GoVersion() string {
	return runtime.Version()
}
