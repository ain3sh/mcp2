package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ain3sh/mcp2/internal/config"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	Long:  `Validate the mcp2 configuration file for errors and inconsistencies.`,
	RunE:  runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Expand config path
	path := expandPath(configPath)

	fmt.Printf("Validating config file: %s\n", path)

	// Load config
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Expand environment variables
	cfg.ExpandEnvVars()

	// Validate
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	fmt.Println("Configuration is valid!")
	fmt.Printf("  Default profile: %s\n", cfg.DefaultProfile)
	fmt.Printf("  Servers: %d\n", len(cfg.Servers))
	fmt.Printf("  Profiles: %d\n", len(cfg.Profiles))
	fmt.Printf("  Hub enabled: %v\n", cfg.Hub.Enabled)
	fmt.Printf("  Prefix server IDs: %v\n", cfg.Hub.PrefixServerIDs)

	return nil
}

// expandPath expands ~ and environment variables in paths.
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[1:])
		}
	}
	return os.ExpandEnv(path)
}
