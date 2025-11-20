// Package main is the entry point for the mcp2 CLI.
package main

import (
	"fmt"
	"os"

	"github.com/ain3sh/mcp2/cmd/mcp2/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
