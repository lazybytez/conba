// Package main provides the conba CLI entrypoint.
package main

import "log"

var version = "dev"

func versionString() string {
	return "conba v" + version
}

func main() {
	log.Println(versionString())
}
