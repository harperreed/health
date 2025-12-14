// ABOUTME: Entry point for health CLI.
// ABOUTME: Invokes the root Cobra command.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Execute is a placeholder until root.go is created
func Execute() error {
	fmt.Println("health CLI - not yet implemented")
	return nil
}
