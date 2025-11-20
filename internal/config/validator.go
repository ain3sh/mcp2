package config

import (
	"fmt"
)

// Validate checks the configuration for errors and inconsistencies.
func (cfg *RootConfig) Validate() error {
	// Check that default profile exists
	if cfg.DefaultProfile == "" {
		return fmt.Errorf("defaultProfile must be specified")
	}
	if _, ok := cfg.Profiles[cfg.DefaultProfile]; !ok {
		return fmt.Errorf("defaultProfile %q does not exist in profiles", cfg.DefaultProfile)
	}

	// Check that all servers referenced in profiles exist
	for profileName, profile := range cfg.Profiles {
		for serverID := range profile.Servers {
			if _, ok := cfg.Servers[serverID]; !ok {
				return fmt.Errorf("profile %q references unknown server %q", profileName, serverID)
			}
		}
	}

	// Validate server transport configurations
	for serverID, server := range cfg.Servers {
		if err := validateServerConfig(serverID, &server); err != nil {
			return err
		}
	}

	// Check for name collisions if hub is enabled without prefixing
	if cfg.Hub.Enabled && !cfg.Hub.PrefixServerIDs {
		if err := checkNameCollisions(cfg); err != nil {
			return err
		}
	}

	return nil
}

func validateServerConfig(serverID string, server *ServerConfig) error {
	switch server.Transport.Kind {
	case "stdio":
		if server.Transport.Command == "" {
			return fmt.Errorf("server %q: stdio transport requires 'command' to be set", serverID)
		}
	case "http":
		if server.Transport.URL == "" {
			return fmt.Errorf("server %q: http transport requires 'url' to be set", serverID)
		}
	case "":
		return fmt.Errorf("server %q: transport 'kind' must be specified (stdio or http)", serverID)
	default:
		return fmt.Errorf("server %q: unknown transport kind %q (must be 'stdio' or 'http')", serverID, server.Transport.Kind)
	}
	return nil
}

func checkNameCollisions(cfg *RootConfig) error {
	// This is a simplified check. In a full implementation, we would need to
	// actually connect to servers and query their tools/resources/prompts.
	// For now, we just warn that collision detection requires prefix mode.

	// Count servers - if more than 1, recommend prefix mode
	if len(cfg.Servers) > 1 {
		return fmt.Errorf("hub is enabled with multiple servers but prefixServerIDs is false; " +
			"this may cause name collisions. Consider setting hub.prefixServerIDs to true")
	}
	return nil
}
