package profile

import (
	"testing"

	"github.com/ain3sh/mcp2/internal/config"
)

func TestIsToolAllowed_AllowAll(t *testing.T) {
	cfg := &config.RootConfig{
		Profiles: map[string]config.ProfileConfig{
			"test": {
				Servers: map[string]config.ServerProfileConfig{
					"server1": {
						Tools: config.ComponentFilter{
							Allow: []string{}, // Empty allow = allow all
							Deny:  []string{},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(cfg, "test")

	if !engine.IsToolAllowed("server1", "any_tool") {
		t.Error("Expected any_tool to be allowed with empty allow list")
	}
}

func TestIsToolAllowed_DenySpecific(t *testing.T) {
	cfg := &config.RootConfig{
		Profiles: map[string]config.ProfileConfig{
			"test": {
				Servers: map[string]config.ServerProfileConfig{
					"server1": {
						Tools: config.ComponentFilter{
							Allow: []string{}, // Allow all
							Deny:  []string{"dangerous_tool"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(cfg, "test")

	if engine.IsToolAllowed("server1", "dangerous_tool") {
		t.Error("Expected dangerous_tool to be denied")
	}

	if !engine.IsToolAllowed("server1", "safe_tool") {
		t.Error("Expected safe_tool to be allowed")
	}
}

func TestIsToolAllowed_AllowSpecific(t *testing.T) {
	cfg := &config.RootConfig{
		Profiles: map[string]config.ProfileConfig{
			"test": {
				Servers: map[string]config.ServerProfileConfig{
					"server1": {
						Tools: config.ComponentFilter{
							Allow: []string{"read_file", "list_directory"},
							Deny:  []string{},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(cfg, "test")

	if !engine.IsToolAllowed("server1", "read_file") {
		t.Error("Expected read_file to be allowed")
	}

	if !engine.IsToolAllowed("server1", "list_directory") {
		t.Error("Expected list_directory to be allowed")
	}

	if engine.IsToolAllowed("server1", "write_file") {
		t.Error("Expected write_file to be denied (not in allow list)")
	}
}

func TestIsToolAllowed_AllowWithDeny(t *testing.T) {
	cfg := &config.RootConfig{
		Profiles: map[string]config.ProfileConfig{
			"test": {
				Servers: map[string]config.ServerProfileConfig{
					"server1": {
						Tools: config.ComponentFilter{
							Allow: []string{"*"}, // Allow all
							Deny:  []string{"delete_file", "format_disk"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(cfg, "test")

	if !engine.IsToolAllowed("server1", "read_file") {
		t.Error("Expected read_file to be allowed")
	}

	if engine.IsToolAllowed("server1", "delete_file") {
		t.Error("Expected delete_file to be denied")
	}

	if engine.IsToolAllowed("server1", "format_disk") {
		t.Error("Expected format_disk to be denied")
	}
}

func TestIsResourceAllowed_URIPatterns(t *testing.T) {
	cfg := &config.RootConfig{
		Profiles: map[string]config.ProfileConfig{
			"test": {
				Servers: map[string]config.ServerProfileConfig{
					"server1": {
						Resources: config.ComponentFilter{
							Allow: []string{"file://docs/**", "file://public/**"},
							Deny:  []string{"file://docs/secret/**"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(cfg, "test")

	// Should allow files in docs (but not secret subdirectory)
	if !engine.IsResourceAllowed("server1", "file://docs/readme.md") {
		t.Error("Expected file://docs/readme.md to be allowed")
	}

	if engine.IsResourceAllowed("server1", "file://docs/secret/key.txt") {
		t.Error("Expected file://docs/secret/key.txt to be denied")
	}

	// Should allow files in public
	if !engine.IsResourceAllowed("server1", "file://public/index.html") {
		t.Error("Expected file://public/index.html to be allowed")
	}

	// Should deny files not matching allow patterns
	if engine.IsResourceAllowed("server1", "file://private/data.txt") {
		t.Error("Expected file://private/data.txt to be denied (not in allow list)")
	}
}

func TestIsPromptAllowed(t *testing.T) {
	cfg := &config.RootConfig{
		Profiles: map[string]config.ProfileConfig{
			"test": {
				Servers: map[string]config.ServerProfileConfig{
					"server1": {
						Prompts: config.ComponentFilter{
							Allow: []string{"help_*"},
							Deny:  []string{"help_admin"},
						},
					},
				},
			},
		},
	}

	engine := NewEngine(cfg, "test")

	if !engine.IsPromptAllowed("server1", "help_user") {
		t.Error("Expected help_user to be allowed")
	}

	if engine.IsPromptAllowed("server1", "help_admin") {
		t.Error("Expected help_admin to be denied (in deny list)")
	}

	if engine.IsPromptAllowed("server1", "other_prompt") {
		t.Error("Expected other_prompt to be denied (not in allow list)")
	}
}

func TestIsAllowed_ProfileNotFound(t *testing.T) {
	cfg := &config.RootConfig{
		Profiles: map[string]config.ProfileConfig{},
	}

	engine := NewEngine(cfg, "nonexistent")

	if engine.IsToolAllowed("server1", "any_tool") {
		t.Error("Expected tool to be denied when profile doesn't exist")
	}
}

func TestIsAllowed_ServerNotInProfile(t *testing.T) {
	cfg := &config.RootConfig{
		Profiles: map[string]config.ProfileConfig{
			"test": {
				Servers: map[string]config.ServerProfileConfig{},
			},
		},
	}

	engine := NewEngine(cfg, "test")

	if engine.IsToolAllowed("server1", "any_tool") {
		t.Error("Expected tool to be denied when server not in profile")
	}
}

func TestMatchPattern_Wildcards(t *testing.T) {
	tests := []struct {
		name     string
		testName string
		pattern  string
		expected bool
	}{
		{"exact match same", "read_file", "read_file", true},
		{"exact match different", "read_file", "write_file", false},
		{"wildcard *", "read_file", "*", true},
		{"wildcard **", "read_file", "**", true},
		{"prefix glob", "read_file", "read_*", true},
		{"suffix glob", "read_file", "*_file", true},
		{"no match", "read_file", "write_*", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchPattern(tt.testName, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.testName, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestMatchPattern_URIGlobs(t *testing.T) {
	tests := []struct {
		uri      string
		pattern  string
		expected bool
	}{
		{"file://docs/readme.md", "file://docs/**", true},
		{"file://docs/subdir/file.txt", "file://docs/**", true},
		{"file://other/file.txt", "file://docs/**", false},
		{"file://docs/secret/key.txt", "file://docs/secret/**", true},
		{"http://example.com/api", "http://**", true},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			result := matchPattern(tt.uri, tt.pattern)
			if result != tt.expected {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.uri, tt.pattern, result, tt.expected)
			}
		})
	}
}

func TestMatchesAny(t *testing.T) {
	patterns := []string{"read_*", "list_*", "get_*"}

	tests := []struct {
		name     string
		expected bool
	}{
		{"read_file", true},
		{"list_directory", true},
		{"get_config", true},
		{"write_file", false},
		{"delete_file", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesAny(tt.name, patterns)
			if result != tt.expected {
				t.Errorf("matchesAny(%q, patterns) = %v, want %v", tt.name, result, tt.expected)
			}
		})
	}
}
