package profile

import (
	"path/filepath"

	"mcp2/internal/config"
)

type Engine struct {
	Config        *config.RootConfig
	ActiveProfile string
}

func NewEngine(cfg *config.RootConfig, profileName string) *Engine {
	return &Engine{
		Config:        cfg,
		ActiveProfile: profileName,
	}
}

func (e *Engine) IsToolAllowed(serverName, toolName string) bool {
	profile, ok := e.Config.Profiles[e.ActiveProfile]
	if !ok {
		return false
	}

	serverConfig, ok := profile.Servers[serverName]
	if !ok {
		return false
	}

	return isAllowed(serverConfig.Tools, toolName)
}

func (e *Engine) IsResourceAllowed(serverName, uri string) bool {
	profile, ok := e.Config.Profiles[e.ActiveProfile]
	if !ok {
		return false
	}

	serverConfig, ok := profile.Servers[serverName]
	if !ok {
		return false
	}

	return isAllowed(serverConfig.Resources, uri)
}

func (e *Engine) IsPromptAllowed(serverName, promptName string) bool {
	profile, ok := e.Config.Profiles[e.ActiveProfile]
	if !ok {
		return false
	}

	serverConfig, ok := profile.Servers[serverName]
	if !ok {
		return false
	}

	return isAllowed(serverConfig.Prompts, promptName)
}

func isAllowed(filter config.ComponentFilter, name string) bool {
	// Spec says:
	// If allow empty → “allow all except deny.”
	// If allow non-empty → “allow only those, then subtract deny.”

    // First check if denied. Deny subtracts from set.
    // Wait, if allow non-empty, we allow only those.
    // So allow list is primary.
    // Deny list removes from allow list (or from all if allow is empty).

    // So if name matches any Deny, it is disallowed.

	for _, pattern := range filter.Deny {
		if match(pattern, name) {
			return false
		}
	}

	if len(filter.Allow) == 0 {
		return true // Allow all if allow list is empty (and not denied)
	}

	for _, pattern := range filter.Allow {
		if match(pattern, name) {
			return true
		}
	}

	return false
}

func match(pattern, name string) bool {
	// Use filepath.Match for globbing
	matched, err := filepath.Match(pattern, name)
	if err != nil {
		// If pattern is malformed, assume no match?
		return false
	}
	return matched
}
