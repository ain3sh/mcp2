package proxy

import (
	"context"
	"fmt"

	"github.com/ain3sh/mcp2/internal/config"
	"github.com/ain3sh/mcp2/internal/profile"
	"github.com/ain3sh/mcp2/internal/upstream"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// PerServerProxy exposes a single upstream server with profile-based filtering.
// Unlike the Hub, it doesn't aggregate or prefix - it provides direct access to one upstream.
type PerServerProxy struct {
	server        *mcp.Server
	upstream      *upstream.Upstream
	profileEngine *profile.Engine
	serverID      string
}

// NewPerServerProxy creates a proxy for a single upstream server.
func NewPerServerProxy(cfg *config.RootConfig, upstream *upstream.Upstream, profileName string) *PerServerProxy {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    fmt.Sprintf("mcp2-proxy-%s", upstream.ID),
		Version: "0.1.0",
	}, nil)

	proxy := &PerServerProxy{
		server:        server,
		upstream:      upstream,
		profileEngine: profile.NewEngine(cfg, profileName),
		serverID:      upstream.ID,
	}

	// Register handlers for this specific upstream
	proxy.registerHandlers()

	return proxy
}

// Server returns the underlying MCP server.
func (p *PerServerProxy) Server() *mcp.Server {
	return p.server
}

// registerHandlers sets up filtering middleware for a single upstream.
func (p *PerServerProxy) registerHandlers() {
	p.server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			switch method {
			case "tools/list":
				return p.handleToolsList(ctx)
			case "tools/call":
				return p.handleToolsCall(ctx, req)
			case "resources/list":
				return p.handleResourcesList(ctx)
			case "resources/read":
				return p.handleResourcesRead(ctx, req)
			case "prompts/list":
				return p.handlePromptsList(ctx)
			case "prompts/get":
				return p.handlePromptsGet(ctx, req)
			default:
				return next(ctx, method, req)
			}
		}
	})
}

// handleToolsList returns filtered tools from the upstream.
func (p *PerServerProxy) handleToolsList(ctx context.Context) (mcp.Result, error) {
	result, err := p.upstream.Session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Filter tools based on profile
	var filteredTools []*mcp.Tool
	for _, tool := range result.Tools {
		if p.profileEngine.IsToolAllowed(p.serverID, tool.Name) {
			filteredTools = append(filteredTools, tool)
		}
	}

	return &mcp.ListToolsResult{Tools: filteredTools}, nil
}

// handleToolsCall enforces call-phase filtering for tools.
func (p *PerServerProxy) handleToolsCall(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	callReq, ok := req.(*mcp.CallToolRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type for tools/call")
	}

	// Check if tool is allowed by profile
	if !p.profileEngine.IsToolAllowed(p.serverID, callReq.Params.Name) {
		return nil, fmt.Errorf("tool %q is not allowed by profile", callReq.Params.Name)
	}

	// Forward to upstream
	return p.upstream.Session.CallTool(ctx, &mcp.CallToolParams{
		Name:      callReq.Params.Name,
		Arguments: callReq.Params.Arguments,
	})
}

// handleResourcesList returns filtered resources from the upstream.
func (p *PerServerProxy) handleResourcesList(ctx context.Context) (mcp.Result, error) {
	result, err := p.upstream.Session.ListResources(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Filter resources based on profile
	var filteredResources []*mcp.Resource
	for _, resource := range result.Resources {
		if p.profileEngine.IsResourceAllowed(p.serverID, resource.URI) {
			filteredResources = append(filteredResources, resource)
		}
	}

	return &mcp.ListResourcesResult{Resources: filteredResources}, nil
}

// handleResourcesRead enforces call-phase filtering for resources.
func (p *PerServerProxy) handleResourcesRead(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	readReq, ok := req.(*mcp.ReadResourceRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type for resources/read")
	}

	// Check if resource is allowed by profile
	if !p.profileEngine.IsResourceAllowed(p.serverID, readReq.Params.URI) {
		return nil, fmt.Errorf("resource %q is not allowed by profile", readReq.Params.URI)
	}

	// Forward to upstream
	return p.upstream.Session.ReadResource(ctx, &mcp.ReadResourceParams{
		URI: readReq.Params.URI,
	})
}

// handlePromptsList returns filtered prompts from the upstream.
func (p *PerServerProxy) handlePromptsList(ctx context.Context) (mcp.Result, error) {
	result, err := p.upstream.Session.ListPrompts(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Filter prompts based on profile
	var filteredPrompts []*mcp.Prompt
	for _, prompt := range result.Prompts {
		if p.profileEngine.IsPromptAllowed(p.serverID, prompt.Name) {
			filteredPrompts = append(filteredPrompts, prompt)
		}
	}

	return &mcp.ListPromptsResult{Prompts: filteredPrompts}, nil
}

// handlePromptsGet enforces call-phase filtering for prompts.
func (p *PerServerProxy) handlePromptsGet(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	getReq, ok := req.(*mcp.GetPromptRequest)
	if !ok {
		return nil, fmt.Errorf("invalid request type for prompts/get")
	}

	// Check if prompt is allowed by profile
	if !p.profileEngine.IsPromptAllowed(p.serverID, getReq.Params.Name) {
		return nil, fmt.Errorf("prompt %q is not allowed by profile", getReq.Params.Name)
	}

	// Forward to upstream
	return p.upstream.Session.GetPrompt(ctx, &mcp.GetPromptParams{
		Name:      getReq.Params.Name,
		Arguments: getReq.Params.Arguments,
	})
}
