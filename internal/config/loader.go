package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Load reads and parses a configuration file (YAML or JSON).
func Load(path string) (*RootConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg RootConfig

	// Determine format based on file extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		// Try YAML first, then JSON
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			if jsonErr := json.Unmarshal(data, &cfg); jsonErr != nil {
				return nil, fmt.Errorf("failed to parse config (tried both YAML and JSON): YAML: %w, JSON: %w", err, jsonErr)
			}
		}
	}

	return &cfg, nil
}

// ExpandEnvVars expands environment variables in the configuration.
// This is useful for things like ${GITHUB_TOKEN} in headers.
func (cfg *RootConfig) ExpandEnvVars() {
	for serverID, server := range cfg.Servers {
		// Expand environment variables in command
		server.Transport.Command = os.ExpandEnv(server.Transport.Command)

		// Expand in args
		for i, arg := range server.Transport.Args {
			server.Transport.Args[i] = os.ExpandEnv(arg)
		}

		// Expand in env values
		for k, v := range server.Transport.Env {
			server.Transport.Env[k] = os.ExpandEnv(v)
		}

		// Expand in HTTP URL
		server.Transport.URL = os.ExpandEnv(server.Transport.URL)

		// Expand in HTTP headers
		for k, v := range server.Transport.Headers {
			server.Transport.Headers[k] = os.ExpandEnv(v)
		}

		// Write the modified server back to the map
		cfg.Servers[serverID] = server
	}
}
