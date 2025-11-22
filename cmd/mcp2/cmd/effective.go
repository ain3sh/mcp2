package cmd

import (
	"fmt"

	"github.com/ain3sh/mcp2/internal/config"
	"github.com/ain3sh/mcp2/internal/profile"
	"github.com/spf13/cobra"
)

var (
	effectiveServer string
)

var effectiveCmd = &cobra.Command{
	Use:   "effective",
	Short: "Show effective filtering rules for a profile",
	Long:  `Display the effective tools, resources, and prompts that are allowed/denied for a given profile and server.`,
	RunE:  runEffective,
}

func init() {
	rootCmd.AddCommand(effectiveCmd)
	effectiveCmd.Flags().StringVarP(&effectiveServer, "server", "s", "", "server to show effective rules for (required)")
	effectiveCmd.MarkFlagRequired("server")
}

func runEffective(cmd *cobra.Command, args []string) error {
	// Expand config path
	path := expandPath(configPath)

	// Load config
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.ExpandEnvVars()

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Determine active profile
	activeProfile := cfg.DefaultProfile
	if profileName != "" {
		activeProfile = profileName
	}

	profileCfg, ok := cfg.Profiles[activeProfile]
	if !ok {
		return fmt.Errorf("profile %q not found", activeProfile)
	}

	// Check if server exists in config
	_, ok = cfg.Servers[effectiveServer]
	if !ok {
		return fmt.Errorf("server %q not found in config", effectiveServer)
	}

	// Get server profile config
	serverProfile, ok := profileCfg.Servers[effectiveServer]
	if !ok {
		fmt.Printf("Profile: %s\n", activeProfile)
		fmt.Printf("Server: %s\n", effectiveServer)
		fmt.Println("\nServer is not configured in this profile (all access denied)")
		return nil
	}

	// Create profile engine for testing
	engine := profile.NewEngine(cfg, activeProfile)

	fmt.Printf("Profile: %s\n", activeProfile)
	fmt.Printf("Description: %s\n", profileCfg.Description)
	fmt.Printf("Server: %s\n\n", effectiveServer)

	// Display tools filtering
	fmt.Println("Tools:")
	displayFilterRules("  ", serverProfile.Tools, func(name string) bool {
		return engine.IsToolAllowed(effectiveServer, name)
	})

	// Display resources filtering
	fmt.Println("\nResources:")
	displayFilterRules("  ", serverProfile.Resources, func(uri string) bool {
		return engine.IsResourceAllowed(effectiveServer, uri)
	})

	// Display prompts filtering
	fmt.Println("\nPrompts:")
	displayFilterRules("  ", serverProfile.Prompts, func(name string) bool {
		return engine.IsPromptAllowed(effectiveServer, name)
	})

	return nil
}

func displayFilterRules(indent string, filter config.ComponentFilter, testFunc func(string) bool) {
	if len(filter.Allow) == 0 && len(filter.Deny) == 0 {
		fmt.Printf("%sNo filtering rules (allow all)\n", indent)
		return
	}

	if len(filter.Allow) > 0 {
		fmt.Printf("%sAllow:\n", indent)
		for _, pattern := range filter.Allow {
			fmt.Printf("%s  - %s\n", indent, pattern)
		}
	} else {
		fmt.Printf("%sAllow: * (all)\n", indent)
	}

	if len(filter.Deny) > 0 {
		fmt.Printf("%sDeny:\n", indent)
		for _, pattern := range filter.Deny {
			fmt.Printf("%s  - %s\n", indent, pattern)
		}
	}

	// Show examples
	fmt.Printf("\n%sExamples:\n", indent)
	testCases := []string{
		"read_file",
		"write_file",
		"delete_file",
		"list_directory",
	}

	for _, testCase := range testCases {
		allowed := testFunc(testCase)
		status := "DENIED"
		if allowed {
			status = "ALLOWED"
		}
		fmt.Printf("%s  %s: %s\n", indent, testCase, status)
	}
}
