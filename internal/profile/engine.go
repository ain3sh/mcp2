// Package profile implements profile-based filtering for MCP tools, resources, and prompts.
package profile

import (
	"path/filepath"
	"strings"

	"github.com/ain3sh/mcp2/internal/config"
)

// Engine provides policy queries for filtering MCP components based on profiles.
type Engine struct {
	config  *config.RootConfig
	profile string
}

// NewEngine creates a new profile engine.
func NewEngine(cfg *config.RootConfig, profileName string) *Engine {
	return &Engine{
		config:  cfg,
		profile: profileName,
	}
}

// IsToolAllowed checks if a tool is allowed for the given server in the active profile.
func (e *Engine) IsToolAllowed(serverID, toolName string) bool {
	return e.isAllowed(serverID, toolName, func(spc *config.ServerProfileConfig) *config.ComponentFilter {
		return &spc.Tools
	})
}

// IsResourceAllowed checks if a resource URI is allowed for the given server in the active profile.
func (e *Engine) IsResourceAllowed(serverID, uri string) bool {
	return e.isAllowed(serverID, uri, func(spc *config.ServerProfileConfig) *config.ComponentFilter {
		return &spc.Resources
	})
}

// IsPromptAllowed checks if a prompt is allowed for the given server in the active profile.
func (e *Engine) IsPromptAllowed(serverID, promptName string) bool {
	return e.isAllowed(serverID, promptName, func(spc *config.ServerProfileConfig) *config.ComponentFilter {
		return &spc.Prompts
	})
}

// isAllowed implements the core filtering logic.
// Behavior:
// - If allow list is empty: allow all except those in deny list
// - If allow list is non-empty: allow only those matching allow patterns, then subtract deny patterns
func (e *Engine) isAllowed(serverID, name string, getFilter func(*config.ServerProfileConfig) *config.ComponentFilter) bool {
	// Get the profile
	profile, ok := e.config.Profiles[e.profile]
	if !ok {
		// If profile doesn't exist, deny by default
		return false
	}

	// Get the server profile config
	serverProfile, ok := profile.Servers[serverID]
	if !ok {
		// If server not in profile, deny by default
		return false
	}

	// Get the component filter
	filter := getFilter(&serverProfile)

	// Check deny list first
	if matchesAny(name, filter.Deny) {
		return false
	}

	// If allow list is empty, allow everything (except what's denied)
	if len(filter.Allow) == 0 {
		return true
	}

	// If allow list is non-empty, only allow what matches
	return matchesAny(name, filter.Allow)
}

// matchesAny checks if a name matches any pattern in the list.
// Supports glob patterns: *, **, and filepath-style globs.
func matchesAny(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPattern(name, pattern) {
			return true
		}
	}
	return false
}

// matchPattern checks if a name matches a pattern.
// Supports:
// - Exact match
// - "*" wildcard (matches anything)
// - "**" wildcard (matches anything including path separators)
// - Glob patterns using filepath.Match
func matchPattern(name, pattern string) bool {
	// Handle wildcards
	if pattern == "*" || pattern == "**" {
		return true
	}

	// Handle exact match
	if name == pattern {
		return true
	}

	// Handle glob patterns
	if strings.Contains(pattern, "*") {
		// Special handling for ** (matches everything including separators)
		if strings.Contains(pattern, "**") {
			// Convert ** to a prefix match
			if strings.HasSuffix(pattern, "**") {
				prefix := strings.TrimSuffix(pattern, "**")
				return strings.HasPrefix(name, prefix)
			}
			if strings.HasPrefix(pattern, "**") {
				suffix := strings.TrimPrefix(pattern, "**")
				return strings.HasSuffix(name, suffix)
			}
			// Middle ** - try prefix and suffix match
			parts := strings.SplitN(pattern, "**", 2)
			if len(parts) == 2 {
				return strings.HasPrefix(name, parts[0]) && strings.HasSuffix(name, parts[1])
			}
		}

		// Use filepath.Match for single * wildcards
		matched, err := filepath.Match(pattern, name)
		if err != nil {
			// Pattern is invalid, no match
			return false
		}
		return matched
	}

	return false
}
