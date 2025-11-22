// Package proxy implements the MCP proxy hub server.
package proxy

import (
	"context"
	"fmt"
	"strings"

	"github.com/ain3sh/mcp2/internal/config"
	"github.com/ain3sh/mcp2/internal/profile"
	"github.com/ain3sh/mcp2/internal/upstream"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Hub is the central MCP server that aggregates multiple upstreams.
type Hub struct {
	server         *mcp.Server
	manager        *upstream.Manager
	config         *config.RootConfig
	profileEngine  *profile.Engine
	prefixEnabled  bool
}

// NewHub creates a new hub server with profile-based filtering.
func NewHub(cfg *config.RootConfig, manager *upstream.Manager, profileName string) *Hub {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp2-hub",
		Version: "0.1.0",
	}, nil)

	hub := &Hub{
		server:        server,
		manager:       manager,
		config:        cfg,
		profileEngine: profile.NewEngine(cfg, profileName),
		prefixEnabled: cfg.Hub.PrefixServerIDs,
	}

	// Register aggregated tool handler
	hub.registerToolHandlers()
	hub.registerResourceHandlers()
	hub.registerPromptHandlers()

	return hub
}

// Server returns the underlying MCP server.
func (h *Hub) Server() *mcp.Server {
	return h.server
}

// registerToolHandlers sets up tool aggregation and proxying.
func (h *Hub) registerToolHandlers() {
	// Override the default tools/list handler to aggregate from all upstreams
	h.server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			switch method {
			case "tools/list":
				return h.handleToolsList(ctx)
			case "tools/call":
				return h.handleToolsCall(ctx, req)
			default:
				return next(ctx, method, req)
			}
		}
	})
}

// registerResourceHandlers sets up resource aggregation and proxying.
func (h *Hub) registerResourceHandlers() {
	h.server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			switch method {
			case "resources/list":
				return h.handleResourcesList(ctx)
			case "resources/read":
				return h.handleResourcesRead(ctx, req)
			default:
				return next(ctx, method, req)
			}
		}
	})
}

// registerPromptHandlers sets up prompt aggregation and proxying.
func (h *Hub) registerPromptHandlers() {
	h.server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			switch method {
			case "prompts/list":
				return h.handlePromptsList(ctx)
			case "prompts/get":
				return h.handlePromptsGet(ctx, req)
			default:
				return next(ctx, method, req)
			}
		}
	})
}

// handleToolsList aggregates and filters tools from all upstream servers.
func (h *Hub) handleToolsList(ctx context.Context) (mcp.Result, error) {
	var allTools []*mcp.Tool

	for _, u := range h.manager.List() {
		result, err := u.Session.ListTools(ctx, nil)
		if err != nil {
			// Log error but continue with other upstreams
			continue
		}

		for _, tool := range result.Tools {
			// Filter based on profile
			if !h.profileEngine.IsToolAllowed(u.ID, tool.Name) {
				continue
			}

			// Add server prefix if enabled
			if h.prefixEnabled {
				tool.Name = fmt.Sprintf("%s:%s", u.ID, tool.Name)
			}
			allTools = append(allTools, tool)
		}
	}

	return &mcp.ListToolsResult{Tools: allTools}, nil
}

// handleToolsCall routes tool calls to the appropriate upstream.
func (h *Hub) handleToolsCall(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	callReq, ok := req.(*mcp.CallToolRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type for tools/call")
	}

	toolName := callReq.Params.Name
	var serverID string
	var actualToolName string

	if h.prefixEnabled {
		// Parse server:toolname
		parts := strings.SplitN(toolName, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("tool name must be in format 'server:toolname' when prefixing is enabled")
		}
		serverID = parts[0]
		actualToolName = parts[1]
	} else {
		// Without prefixing, try only upstreams where the profile allows this tool
		var lastErr error
		for _, u := range h.manager.List() {
			if !h.profileEngine.IsToolAllowed(u.ID, toolName) {
				continue
			}
			result, err := u.Session.CallTool(ctx, &mcp.CallToolParams{
				Name:      toolName,
				Arguments: callReq.Params.Arguments,
			})
			if err == nil {
				return result, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, fmt.Errorf("tool %q allowed by profile but call failed: %v", toolName, lastErr)
		}
		return nil, fmt.Errorf("tool %q not found in any upstream or not allowed by profile", toolName)
	}

	// Get the upstream server
	u, err := h.manager.Get(serverID)
	if err != nil {
		return nil, fmt.Errorf("upstream server %q not found", serverID)
	}

	// Check if tool is allowed by profile (call-phase check)
	if !h.profileEngine.IsToolAllowed(serverID, actualToolName) {
		return nil, fmt.Errorf("tool %q is not allowed by profile", toolName)
	}

	// Call the tool on the upstream
	return u.Session.CallTool(ctx, &mcp.CallToolParams{
		Name:      actualToolName,
		Arguments: callReq.Params.Arguments,
	})
}

// handleResourcesList aggregates and filters resources from all upstream servers.
func (h *Hub) handleResourcesList(ctx context.Context) (mcp.Result, error) {
	var allResources []*mcp.Resource

	for _, u := range h.manager.List() {
		result, err := u.Session.ListResources(ctx, nil)
		if err != nil {
			continue
		}

		for _, resource := range result.Resources {
			// Filter based on profile
			if !h.profileEngine.IsResourceAllowed(u.ID, resource.URI) {
				continue
			}

			// Prefix URI if needed
			if h.prefixEnabled {
				resource.URI = fmt.Sprintf("%s:%s", u.ID, resource.URI)
			}
			allResources = append(allResources, resource)
		}
	}

	return &mcp.ListResourcesResult{Resources: allResources}, nil
}

// handleResourcesRead routes resource reads to the appropriate upstream.
func (h *Hub) handleResourcesRead(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	readReq, ok := req.(*mcp.ReadResourceRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type for resources/read")
	}

	uri := readReq.Params.URI
	var serverID string
	var actualURI string

	if h.prefixEnabled {
		parts := strings.SplitN(uri, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("resource URI must be in format 'server:uri' when prefixing is enabled")
		}
		serverID = parts[0]
		actualURI = parts[1]
	} else {
		// Try only upstreams where the profile allows this resource
		var lastErr error
		for _, u := range h.manager.List() {
			if !h.profileEngine.IsResourceAllowed(u.ID, uri) {
				continue
			}
			result, err := u.Session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
			if err == nil {
				return result, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, fmt.Errorf("resource %q allowed by profile but read failed: %v", uri, lastErr)
		}
		return nil, fmt.Errorf("resource %q not found in any upstream or not allowed by profile", uri)
	}

	u, err := h.manager.Get(serverID)
	if err != nil {
		return nil, err
	}

	// Check if resource is allowed by profile (call-phase check)
	if !h.profileEngine.IsResourceAllowed(serverID, actualURI) {
		return nil, fmt.Errorf("resource %q is not allowed by profile", uri)
	}

	return u.Session.ReadResource(ctx, &mcp.ReadResourceParams{URI: actualURI})
}

// handlePromptsList aggregates and filters prompts from all upstream servers.
func (h *Hub) handlePromptsList(ctx context.Context) (mcp.Result, error) {
	var allPrompts []*mcp.Prompt

	for _, u := range h.manager.List() {
		result, err := u.Session.ListPrompts(ctx, nil)
		if err != nil {
			continue
		}

		for _, prompt := range result.Prompts {
			// Filter based on profile
			if !h.profileEngine.IsPromptAllowed(u.ID, prompt.Name) {
				continue
			}

			if h.prefixEnabled {
				prompt.Name = fmt.Sprintf("%s:%s", u.ID, prompt.Name)
			}
			allPrompts = append(allPrompts, prompt)
		}
	}

	return &mcp.ListPromptsResult{Prompts: allPrompts}, nil
}

// handlePromptsGet routes prompt gets to the appropriate upstream.
func (h *Hub) handlePromptsGet(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	getReq, ok := req.(*mcp.GetPromptRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type for prompts/get")
	}

	promptName := getReq.Params.Name
	var serverID string
	var actualPromptName string

	if h.prefixEnabled {
		parts := strings.SplitN(promptName, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("prompt name must be in format 'server:promptname' when prefixing is enabled")
		}
		serverID = parts[0]
		actualPromptName = parts[1]
	} else {
		// Try only upstreams where the profile allows this prompt
		var lastErr error
		for _, u := range h.manager.List() {
			if !h.profileEngine.IsPromptAllowed(u.ID, promptName) {
				continue
			}
			result, err := u.Session.GetPrompt(ctx, &mcp.GetPromptParams{
				Name:      promptName,
				Arguments: getReq.Params.Arguments,
			})
			if err == nil {
				return result, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, fmt.Errorf("prompt %q allowed by profile but get failed: %v", promptName, lastErr)
		}
		return nil, fmt.Errorf("prompt %q not found in any upstream or not allowed by profile", promptName)
	}

	u, err := h.manager.Get(serverID)
	if err != nil {
		return nil, err
	}

	// Check if prompt is allowed by profile (call-phase check)
	if !h.profileEngine.IsPromptAllowed(serverID, actualPromptName) {
		return nil, fmt.Errorf("prompt %q is not allowed by profile", promptName)
	}

	return u.Session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      actualPromptName,
		Arguments: getReq.Params.Arguments,
	})
}
