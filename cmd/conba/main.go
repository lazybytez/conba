// Package main provides the conba CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/lazybytez/conba/internal/cli"
)

func main() {
	err := cli.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
