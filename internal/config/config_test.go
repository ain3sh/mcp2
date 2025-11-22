package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadYAML(t *testing.T) {
	// Create a temporary YAML config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
defaultProfile: safe

servers:
  testserver:
    displayName: "Test Server"
    transport:
      kind: stdio
      command: echo
      args: ["hello"]
      env:
        FOO: bar

profiles:
  safe:
    description: "Safe profile"
    servers:
      testserver:
        tools:
          allow: ["*"]
        resources: {}
        prompts: {}

hub:
  enabled: true
  prefixServerIDs: true

exposePerServer: false
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load the config
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify the loaded config
	if cfg.DefaultProfile != "safe" {
		t.Errorf("DefaultProfile = %q, want %q", cfg.DefaultProfile, "safe")
	}

	if len(cfg.Servers) != 1 {
		t.Errorf("len(Servers) = %d, want 1", len(cfg.Servers))
	}

	server, ok := cfg.Servers["testserver"]
	if !ok {
		t.Fatal("Server 'testserver' not found")
	}

	if server.DisplayName != "Test Server" {
		t.Errorf("DisplayName = %q, want %q", server.DisplayName, "Test Server")
	}

	if server.Transport.Kind != "stdio" {
		t.Errorf("Transport.Kind = %q, want %q", server.Transport.Kind, "stdio")
	}

	if server.Transport.Command != "echo" {
		t.Errorf("Transport.Command = %q, want %q", server.Transport.Command, "echo")
	}

	if cfg.Hub.Enabled != true {
		t.Error("Hub.Enabled should be true")
	}

	if cfg.Hub.PrefixServerIDs != true {
		t.Error("Hub.PrefixServerIDs should be true")
	}
}

func TestLoadJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	configContent := `{
  "defaultProfile": "dev",
  "servers": {
    "jsonserver": {
      "displayName": "JSON Server",
      "transport": {
        "kind": "http",
        "url": "http://localhost:8000/mcp"
      }
    }
  },
  "profiles": {
    "dev": {
      "description": "Dev profile",
      "servers": {
        "jsonserver": {
          "tools": {},
          "resources": {},
          "prompts": {}
        }
      }
    }
  },
  "hub": {
    "enabled": true,
    "prefixServerIDs": false
  },
  "exposePerServer": true
}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.DefaultProfile != "dev" {
		t.Errorf("DefaultProfile = %q, want %q", cfg.DefaultProfile, "dev")
	}

	server, ok := cfg.Servers["jsonserver"]
	if !ok {
		t.Fatal("Server 'jsonserver' not found")
	}

	if server.Transport.Kind != "http" {
		t.Errorf("Transport.Kind = %q, want %q", server.Transport.Kind, "http")
	}

	if server.Transport.URL != "http://localhost:8000/mcp" {
		t.Errorf("Transport.URL = %q, want %q", server.Transport.URL, "http://localhost:8000/mcp")
	}
}

func TestValidate_Success(t *testing.T) {
	cfg := &RootConfig{
		DefaultProfile: "test",
		Servers: map[string]ServerConfig{
			"server1": {
				DisplayName: "Server 1",
				Transport: ServerTransportConfig{
					Kind:    "stdio",
					Command: "test",
				},
			},
		},
		Profiles: map[string]ProfileConfig{
			"test": {
				Description: "Test profile",
				Servers: map[string]ServerProfileConfig{
					"server1": {},
				},
			},
		},
		Hub: HubConfig{
			Enabled:         true,
			PrefixServerIDs: true,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() failed: %v", err)
	}
}

func TestValidate_MissingDefaultProfile(t *testing.T) {
	cfg := &RootConfig{
		DefaultProfile: "",
		Servers:        map[string]ServerConfig{},
		Profiles:       map[string]ProfileConfig{},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for missing defaultProfile, got nil")
	}
}

func TestValidate_DefaultProfileNotFound(t *testing.T) {
	cfg := &RootConfig{
		DefaultProfile: "nonexistent",
		Servers:        map[string]ServerConfig{},
		Profiles:       map[string]ProfileConfig{},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for nonexistent defaultProfile, got nil")
	}
}

func TestValidate_UnknownServerInProfile(t *testing.T) {
	cfg := &RootConfig{
		DefaultProfile: "test",
		Servers:        map[string]ServerConfig{},
		Profiles: map[string]ProfileConfig{
			"test": {
				Servers: map[string]ServerProfileConfig{
					"unknownserver": {},
				},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for unknown server in profile, got nil")
	}
}

func TestValidate_InvalidTransportKind(t *testing.T) {
	cfg := &RootConfig{
		DefaultProfile: "test",
		Servers: map[string]ServerConfig{
			"server1": {
				Transport: ServerTransportConfig{
					Kind: "invalid",
				},
			},
		},
		Profiles: map[string]ProfileConfig{
			"test": {},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for invalid transport kind, got nil")
	}
}

func TestValidate_StdioMissingCommand(t *testing.T) {
	cfg := &RootConfig{
		DefaultProfile: "test",
		Servers: map[string]ServerConfig{
			"server1": {
				Transport: ServerTransportConfig{
					Kind: "stdio",
					// Command is missing
				},
			},
		},
		Profiles: map[string]ProfileConfig{
			"test": {},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for stdio without command, got nil")
	}
}

func TestValidate_HTTPMissingURL(t *testing.T) {
	cfg := &RootConfig{
		DefaultProfile: "test",
		Servers: map[string]ServerConfig{
			"server1": {
				Transport: ServerTransportConfig{
					Kind: "http",
					// URL is missing
				},
			},
		},
		Profiles: map[string]ProfileConfig{
			"test": {},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for http without URL, got nil")
	}
}

func TestExpandEnvVars(t *testing.T) {
	// Set test environment variable
	os.Setenv("TEST_TOKEN", "secret123")
	defer os.Unsetenv("TEST_TOKEN")

	cfg := &RootConfig{
		Servers: map[string]ServerConfig{
			"server1": {
				Transport: ServerTransportConfig{
					Kind:    "http",
					URL:     "http://localhost:8000",
					Headers: map[string]string{
						"Authorization": "Bearer ${TEST_TOKEN}",
					},
					Env: map[string]string{
						"API_KEY": "${TEST_TOKEN}",
					},
				},
			},
		},
	}

	cfg.ExpandEnvVars()

	server := cfg.Servers["server1"]
	if server.Transport.Headers["Authorization"] != "Bearer secret123" {
		t.Errorf("Headers not expanded: got %q", server.Transport.Headers["Authorization"])
	}

	if server.Transport.Env["API_KEY"] != "secret123" {
		t.Errorf("Env not expanded: got %q", server.Transport.Env["API_KEY"])
	}
}

func TestLoad_NonexistentFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	// Use truly invalid YAML with syntax errors
	invalidContent := `
defaultProfile: [unclosed bracket
servers:
  - invalid: structure: here
`

	if err := os.WriteFile(configPath, []byte(invalidContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}
