// Package upstream manages connections to upstream MCP servers.
package upstream

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"github.com/ain3sh/mcp2/internal/config"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Upstream represents a connection to an upstream MCP server.
type Upstream struct {
	ID          string
	DisplayName string
	Session     *mcp.ClientSession
	Config      *config.ServerConfig
}

// Manager manages multiple upstream MCP server connections.
type Manager struct {
	upstreams map[string]*Upstream
	mu        sync.RWMutex
}

// NewManager creates a new upstream manager.
func NewManager() *Manager {
	return &Manager{
		upstreams: make(map[string]*Upstream),
	}
}

// Connect establishes a connection to an upstream server.
func (m *Manager) Connect(ctx context.Context, serverID string, serverCfg *config.ServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already connected
	if _, exists := m.upstreams[serverID]; exists {
		return fmt.Errorf("already connected to server %q", serverID)
	}

	// Create MCP client
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "mcp2-proxy",
		Version: "0.1.0",
	}, nil)

	// Create transport based on config
	var transport mcp.Transport
	var err error

	switch serverCfg.Transport.Kind {
	case "stdio":
		transport, err = createStdioTransport(serverCfg)
	case "http":
		transport, err = createHTTPTransport(serverCfg)
	default:
		return fmt.Errorf("unsupported transport kind: %q", serverCfg.Transport.Kind)
	}

	if err != nil {
		return fmt.Errorf("failed to create transport for server %q: %w", serverID, err)
	}

	// Connect to the upstream server
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to server %q: %w", serverID, err)
	}

	// Store the upstream
	m.upstreams[serverID] = &Upstream{
		ID:          serverID,
		DisplayName: serverCfg.DisplayName,
		Session:     session,
		Config:      serverCfg,
	}

	return nil
}

// Get retrieves an upstream by ID.
func (m *Manager) Get(serverID string) (*Upstream, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	upstream, ok := m.upstreams[serverID]
	if !ok {
		return nil, fmt.Errorf("upstream server %q not found", serverID)
	}
	return upstream, nil
}

// List returns all upstreams.
func (m *Manager) List() []*Upstream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Upstream, 0, len(m.upstreams))
	for _, u := range m.upstreams {
		result = append(result, u)
	}
	return result
}

// Close closes all upstream connections.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error
	for id, upstream := range m.upstreams {
		if err := upstream.Session.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close upstream %q: %w", id, err))
		}
	}

	// Clear the upstreams map to allow future reconnects
	m.upstreams = make(map[string]*Upstream)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing upstreams: %v", errs)
	}
	return nil
}

// createStdioTransport creates a stdio transport for an upstream server.
func createStdioTransport(serverCfg *config.ServerConfig) (mcp.Transport, error) {
	cmd := exec.Command(serverCfg.Transport.Command, serverCfg.Transport.Args...)

	// Set environment variables
	if len(serverCfg.Transport.Env) > 0 {
		env := cmd.Environ()
		for k, v := range serverCfg.Transport.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	return &mcp.CommandTransport{Command: cmd}, nil
}

// createHTTPTransport creates an HTTP transport for an upstream server.
func createHTTPTransport(serverCfg *config.ServerConfig) (mcp.Transport, error) {
	// Use StreamableClientTransport for HTTP
	return &mcp.StreamableClientTransport{
		Endpoint: serverCfg.Transport.URL,
		// TODO: Add support for custom headers via middleware or transport options
	}, nil
}
