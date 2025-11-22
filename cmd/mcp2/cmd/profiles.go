package cmd

import (
	"fmt"
	"sort"

	"github.com/ain3sh/mcp2/internal/config"
	"github.com/spf13/cobra"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "List available profiles",
	Long: `List all profiles defined in the configuration file with their descriptions.
Shows which profile is configured as the default.`,
	RunE: runProfiles,
}

func init() {
	rootCmd.AddCommand(profilesCmd)
}

func runProfiles(cmd *cobra.Command, args []string) error {
	// Expand config path
	path := expandPath(configPath)

	// Load config
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Expand environment variables
	cfg.ExpandEnvVars()

	// Validate config
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Print header
	fmt.Printf("Available Profiles\n")
	fmt.Printf("==================\n\n")

	// Get sorted profile names for consistent output
	profileNames := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		profileNames = append(profileNames, name)
	}
	sort.Strings(profileNames)

	// Print each profile
	for _, name := range profileNames {
		profile := cfg.Profiles[name]

		// Mark default profile
		defaultMarker := ""
		if name == cfg.DefaultProfile {
			defaultMarker = " (default)"
		}

		fmt.Printf("Profile: %s%s\n", name, defaultMarker)

		if profile.Description != "" {
			fmt.Printf("  Description: %s\n", profile.Description)
		}

		// Count configured servers for this profile
		serverCount := len(profile.Servers)
		fmt.Printf("  Servers: %d configured\n", serverCount)

		// Show server names
		if serverCount > 0 {
			serverNames := make([]string, 0, serverCount)
			for serverID := range profile.Servers {
				serverNames = append(serverNames, serverID)
			}
			sort.Strings(serverNames)

			for _, serverID := range serverNames {
				serverCfg := profile.Servers[serverID]
				displayName := serverID
				if globalServerCfg, exists := cfg.Servers[serverID]; exists && globalServerCfg.DisplayName != "" {
					displayName = globalServerCfg.DisplayName
				}

				// Count filters
				toolFilters := len(serverCfg.Tools.Allow) + len(serverCfg.Tools.Deny)
				resourceFilters := len(serverCfg.Resources.Allow) + len(serverCfg.Resources.Deny)
				promptFilters := len(serverCfg.Prompts.Allow) + len(serverCfg.Prompts.Deny)

				filterInfo := ""
				if toolFilters > 0 || resourceFilters > 0 || promptFilters > 0 {
					parts := []string{}
					if toolFilters > 0 {
						parts = append(parts, fmt.Sprintf("%d tool filter(s)", toolFilters))
					}
					if resourceFilters > 0 {
						parts = append(parts, fmt.Sprintf("%d resource filter(s)", resourceFilters))
					}
					if promptFilters > 0 {
						parts = append(parts, fmt.Sprintf("%d prompt filter(s)", promptFilters))
					}
					filterInfo = " - " + fmt.Sprintf("%v", parts[0])
					if len(parts) > 1 {
						for i := 1; i < len(parts); i++ {
							filterInfo += ", " + parts[i]
						}
					}
				}

				fmt.Printf("    - %s (%s)%s\n", displayName, serverID, filterInfo)
			}
		}

		fmt.Println()
	}

	// Print summary
	fmt.Printf("Total: %d profile(s)\n", len(cfg.Profiles))
	fmt.Printf("Default profile: %s\n", cfg.DefaultProfile)

	return nil
}
