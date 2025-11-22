// Package cmd implements the CLI commands for mcp2.
package cmd

import (
	"github.com/spf13/cobra"
)

var (
	configPath string
	profileName string
)

var rootCmd = &cobra.Command{
	Use:   "mcp2",
	Short: "MCP proxy with profile-based filtering",
	Long: `mcp2 is a Go-based MCP proxy that sits between MCP clients and upstream servers,
providing profile-based filtering of tools, resources, and prompts.`,
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "~/.config/mcp2/config.yaml", "path to config file")
	rootCmd.PersistentFlags().StringVarP(&profileName, "profile", "p", "", "profile to use (overrides config default)")
}
