// Package config handles configuration loading and validation for mcp2.
package config

// ComponentFilter defines allow/deny rules for tools, resources, or prompts.
type ComponentFilter struct {
	Allow []string `json:"allow" yaml:"allow"` // names or globs
	Deny  []string `json:"deny" yaml:"deny"`
}

// ServerProfileConfig defines per-server filtering rules for a profile.
type ServerProfileConfig struct {
	Tools     ComponentFilter `json:"tools" yaml:"tools"`
	Resources ComponentFilter `json:"resources" yaml:"resources"`
	Prompts   ComponentFilter `json:"prompts" yaml:"prompts"`
}

// ServerTransportConfig defines how to connect to an upstream MCP server.
type ServerTransportConfig struct {
	// Kind is either "stdio" or "http"
	Kind string `json:"kind" yaml:"kind"`

	// For stdio transport
	Command string            `json:"command" yaml:"command"`
	Args    []string          `json:"args" yaml:"args"`
	Env     map[string]string `json:"env" yaml:"env"`

	// For HTTP transport (Streamable HTTP / SSE)
	URL     string            `json:"url" yaml:"url"`
	Headers map[string]string `json:"headers" yaml:"headers"`
}

// ServerConfig defines an upstream MCP server.
type ServerConfig struct {
	DisplayName string                `json:"displayName" yaml:"displayName"`
	Transport   ServerTransportConfig `json:"transport" yaml:"transport"`
}

// ProfileConfig defines a profile with per-server filtering rules.
type ProfileConfig struct {
	Description string                         `json:"description" yaml:"description"`
	Servers     map[string]ServerProfileConfig `json:"servers" yaml:"servers"`
}

// HubConfig defines hub behavior.
type HubConfig struct {
	Enabled         bool `json:"enabled" yaml:"enabled"`
	PrefixServerIDs bool `json:"prefixServerIDs" yaml:"prefixServerIDs"`
}

// RootConfig is the top-level configuration structure.
type RootConfig struct {
	DefaultProfile  string                   `json:"defaultProfile" yaml:"defaultProfile"`
	Servers         map[string]ServerConfig  `json:"servers" yaml:"servers"`
	Profiles        map[string]ProfileConfig `json:"profiles" yaml:"profiles"`
	Hub             HubConfig                `json:"hub" yaml:"hub"`
	ExposePerServer bool                     `json:"exposePerServer" yaml:"exposePerServer"`
}
