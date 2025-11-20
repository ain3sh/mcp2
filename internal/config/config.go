package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ComponentFilter struct {
	Allow []string `json:"allow" yaml:"allow"` // names or globs
	Deny  []string `json:"deny"  yaml:"deny"`
}

type ServerProfileConfig struct {
	Tools     ComponentFilter `json:"tools"     yaml:"tools"`
	Resources ComponentFilter `json:"resources" yaml:"resources"`
	Prompts   ComponentFilter `json:"prompts"   yaml:"prompts"`
}

type ServerTransportConfig struct {
	// "stdio" or "http"
	Kind string `json:"kind" yaml:"kind"`

	// For stdio
	Command string            `json:"command" yaml:"command"`
	Args    []string          `json:"args"    yaml:"args"`
	Env     map[string]string `json:"env"     yaml:"env"`

	// For HTTP (Streamable HTTP / SSE)
	URL     string            `json:"url"     yaml:"url"`
	Headers map[string]string `json:"headers" yaml:"headers"`
}

type ServerConfig struct {
	DisplayName string                `json:"displayName" yaml:"displayName"`
	Transport   ServerTransportConfig `json:"transport"   yaml:"transport"`
}

type ProfileConfig struct {
	Description string                         `json:"description" yaml:"description"`
	Servers     map[string]ServerProfileConfig `json:"servers"     yaml:"servers"`
}

type HubConfig struct {
	Enabled         bool `json:"enabled" yaml:"enabled"`
	PrefixServerIDs bool `json:"prefixServerIDs" yaml:"prefixServerIDs"`
}

type RootConfig struct {
	DefaultProfile string                   `json:"defaultProfile"   yaml:"defaultProfile"`
	Servers        map[string]ServerConfig  `json:"servers"          yaml:"servers"`
	Profiles       map[string]ProfileConfig `json:"profiles"         yaml:"profiles"`

	// Hub behavior
	Hub HubConfig `json:"hub" yaml:"hub"`

	// Optional per-server HTTP endpoints
	ExposePerServer bool `json:"exposePerServer" yaml:"exposePerServer"`
}

func LoadConfig(path string) (*RootConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg RootConfig
	ext := filepath.Ext(path)
	if ext == ".json" {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	}

	return &cfg, nil
}

func (c *RootConfig) Validate() error {
	if c.DefaultProfile == "" {
		return fmt.Errorf("defaultProfile is required")
	}
	if _, ok := c.Profiles[c.DefaultProfile]; !ok {
		return fmt.Errorf("defaultProfile %q not found in profiles", c.DefaultProfile)
	}

	for name, profile := range c.Profiles {
		for serverName := range profile.Servers {
			if _, ok := c.Servers[serverName]; !ok {
				return fmt.Errorf("profile %q references unknown server %q", name, serverName)
			}
		}
	}

	return nil
}
