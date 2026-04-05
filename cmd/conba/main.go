// Package main provides the conba CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/lazybytez/conba/internal/build"
)

func main() {
	_, _ = fmt.Fprintf(
		os.Stdout,
		"conba %s (go: %s, restic: %s)\n",
		build.ComputeVersionString(),
		build.GoVersion(),
		build.ResticVersion,
	)
}
