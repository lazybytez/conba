// Package main provides the conba CLI entrypoint.
package main

import (
	"os"

	"github.com/lazybytez/conba/internal/cli"
)

func main() {
	os.Exit(cli.Main())
}
