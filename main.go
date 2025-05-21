package main

import (
	"fmt"
	"os"

	"github.com/isaaguilar/infrakube-cli/cmd"
)

var version string

func main() {
	cmd.Execute(version)
}

func init() {
	if version == "" {
		version = "v0.0.0"
	}
	// Hijack the version sub-command. This prevents version from initializing
	// cobra. When more than a single arg of "version" is called,
	// cobra's version implementation will be processed.
	if len(os.Args) == 2 {
		if os.Args[1] == "version" {
			fmt.Println(version)
			os.Exit(0)
		}
	}
}
