// Package main provides the conba CLI entrypoint.
package main

import "fmt"

var version = "dev"

func versionString() string {
	return "conba v" + version
}

func main() {
	fmt.Println(versionString())
}
