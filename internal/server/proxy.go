package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"mcp2/internal/profile"
	"mcp2/internal/upstream"
)

type Proxy struct {
	UpstreamName  string
	Upstream      *upstream.Upstream
	ProfileEngine *profile.Engine
	Server        *mcp.Server
}

func NewProxy(name string, u *upstream.Upstream, pe *profile.Engine) *Proxy {
	p := &Proxy{
		UpstreamName:  name,
		Upstream:      u,
		ProfileEngine: pe,
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp2-proxy-" + name,
		Version: "0.1.0",
	}, nil)

	server.AddReceivingMiddleware(p.middleware)

	p.Server = server
	return p
}

func (p *Proxy) middleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		switch method {
		case "tools/list":
			return p.handleListTools(ctx, req)
		case "tools/call":
			return p.handleCallTool(ctx, req)
		case "resources/list":
			return p.handleListResources(ctx, req)
		case "resources/read":
			return p.handleReadResource(ctx, req)
		case "prompts/list":
			return p.handleListPrompts(ctx, req)
		case "prompts/get":
			return p.handleGetPrompt(ctx, req)
		}
		return next(ctx, method, req)
	}
}

func (p *Proxy) handleListTools(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	if p.Upstream == nil || p.Upstream.Session == nil {
		return &mcp.ListToolsResult{}, nil
	}

	result, err := p.Upstream.Session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, err
	}

	var allowedTools []*mcp.Tool
	for _, tool := range result.Tools {
		if p.ProfileEngine.IsToolAllowed(p.UpstreamName, tool.Name) {
			allowedTools = append(allowedTools, tool)
		}
	}

	return &mcp.ListToolsResult{
		Tools: allowedTools,
	}, nil
}

func (p *Proxy) handleCallTool(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	r, ok := req.(*mcp.CallToolRequest)
	if !ok {
		return nil, fmt.Errorf("unexpected request type: %T", req)
	}
	params := r.Params

	if !p.ProfileEngine.IsToolAllowed(p.UpstreamName, params.Name) {
		return nil, fmt.Errorf("tool disabled by policy")
	}

	var args map[string]any
	if len(params.Arguments) > 0 {
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
		}
	}

	newParams := &mcp.CallToolParams{
		Name:      params.Name,
		Arguments: args,
	}

	return p.Upstream.Session.CallTool(ctx, newParams)
}

func (p *Proxy) handleListResources(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	if p.Upstream == nil || p.Upstream.Session == nil {
		return &mcp.ListResourcesResult{}, nil
	}

	result, err := p.Upstream.Session.ListResources(ctx, &mcp.ListResourcesParams{})
	if err != nil {
		return nil, err
	}

	var allowed []*mcp.Resource
	for _, res := range result.Resources {
		if p.ProfileEngine.IsResourceAllowed(p.UpstreamName, res.URI) {
			allowed = append(allowed, res)
		}
	}

	return &mcp.ListResourcesResult{
		Resources: allowed,
	}, nil
}

func (p *Proxy) handleReadResource(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	r, ok := req.(*mcp.ReadResourceRequest)
	if !ok {
		return nil, fmt.Errorf("unexpected request type: %T", req)
	}
	params := r.Params

	if !p.ProfileEngine.IsResourceAllowed(p.UpstreamName, params.URI) {
		return nil, fmt.Errorf("resource disabled by policy")
	}

	return p.Upstream.Session.ReadResource(ctx, params)
}

func (p *Proxy) handleListPrompts(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	if p.Upstream == nil || p.Upstream.Session == nil {
		return &mcp.ListPromptsResult{}, nil
	}

	result, err := p.Upstream.Session.ListPrompts(ctx, &mcp.ListPromptsParams{})
	if err != nil {
		return nil, err
	}

	var allowed []*mcp.Prompt
	for _, prompt := range result.Prompts {
		if p.ProfileEngine.IsPromptAllowed(p.UpstreamName, prompt.Name) {
			allowed = append(allowed, prompt)
		}
	}

	return &mcp.ListPromptsResult{
		Prompts: allowed,
	}, nil
}

func (p *Proxy) handleGetPrompt(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	r, ok := req.(*mcp.GetPromptRequest)
	if !ok {
		return nil, fmt.Errorf("unexpected request type: %T", req)
	}
	params := r.Params

	if !p.ProfileEngine.IsPromptAllowed(p.UpstreamName, params.Name) {
		return nil, fmt.Errorf("prompt disabled by policy")
	}

	return p.Upstream.Session.GetPrompt(ctx, params)
}
