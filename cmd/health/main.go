// ABOUTME: Entry point for health CLI.
// ABOUTME: Invokes the root Cobra command.
package main

import (
	"fmt"
	"os"
)

// version is stamped by goreleaser via -X main.version at release build.
var version = "dev"

func init() {
	rootCmd.Version = version
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
