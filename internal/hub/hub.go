package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"mcp2/internal/config"
	"mcp2/internal/profile"
	"mcp2/internal/upstream"
)

type Hub struct {
	Config          *config.RootConfig
	UpstreamManager *upstream.Manager
	Server          *mcp.Server
	ProfileEngine   *profile.Engine
}

func NewHub(cfg *config.RootConfig, mgr *upstream.Manager, pe *profile.Engine) *Hub {
	h := &Hub{
		Config:          cfg,
		UpstreamManager: mgr,
		ProfileEngine:   pe,
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp2-hub",
		Version: "0.1.0",
	}, nil)

	server.AddReceivingMiddleware(h.middleware)

	h.Server = server
	return h
}

func (h *Hub) middleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		switch method {
		case "tools/list":
			return h.handleListTools(ctx, req)
		case "tools/call":
			return h.handleCallTool(ctx, req)
		case "resources/list":
			return h.handleListResources(ctx, req)
		case "resources/read":
			return h.handleReadResource(ctx, req)
		case "prompts/list":
			return h.handleListPrompts(ctx, req)
		case "prompts/get":
			return h.handleGetPrompt(ctx, req)
		}
		return next(ctx, method, req)
	}
}

func (h *Hub) handleListTools(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	var allTools []*mcp.Tool

	for name := range h.Config.Servers {
		u := h.UpstreamManager.Get(name)
		if u == nil || u.Session == nil {
			continue
		}

		result, err := u.Session.ListTools(ctx, &mcp.ListToolsParams{})
		if err != nil {
			continue
		}

		for _, tool := range result.Tools {
			if !h.ProfileEngine.IsToolAllowed(name, tool.Name) {
				continue
			}
			if h.Config.Hub.PrefixServerIDs {
				t := *tool
				t.Name = fmt.Sprintf("%s:%s", name, tool.Name)
				allTools = append(allTools, &t)
			} else {
				allTools = append(allTools, tool)
			}
		}
	}

	return &mcp.ListToolsResult{
		Tools: allTools,
	}, nil
}

func (h *Hub) handleCallTool(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	r, ok := req.(*mcp.CallToolRequest)
	if !ok {
		return nil, fmt.Errorf("unexpected request type for tools/call: %T", req)
	}
	params := r.Params

	serverName, toolName, err := h.parseName(params.Name)
	if err != nil {
		return nil, err
	}

	u := h.UpstreamManager.Get(serverName)
	if u == nil || u.Session == nil {
		return nil, fmt.Errorf("server %q not found or not connected", serverName)
	}

	if !h.ProfileEngine.IsToolAllowed(serverName, toolName) {
		return nil, fmt.Errorf("tool disabled by policy")
	}

	var args map[string]any
	if len(params.Arguments) > 0 {
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
		}
	}

	newParams := &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	}

	return u.Session.CallTool(ctx, newParams)
}

func (h *Hub) handleListResources(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	var allResources []*mcp.Resource

	for name := range h.Config.Servers {
		u := h.UpstreamManager.Get(name)
		if u == nil || u.Session == nil {
			continue
		}

		result, err := u.Session.ListResources(ctx, &mcp.ListResourcesParams{})
		if err != nil {
			continue
		}

		for _, res := range result.Resources {
			if !h.ProfileEngine.IsResourceAllowed(name, res.URI) {
				continue
			}
			allResources = append(allResources, res)
		}
	}

	return &mcp.ListResourcesResult{
		Resources: allResources,
	}, nil
}

func (h *Hub) handleReadResource(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	r, ok := req.(*mcp.ReadResourceRequest)
	if !ok {
		return nil, fmt.Errorf("unexpected request type for resources/read: %T", req)
	}
	params := r.Params

	for name := range h.Config.Servers {
		u := h.UpstreamManager.Get(name)
		if u == nil || u.Session == nil {
			continue
		}

		if !h.ProfileEngine.IsResourceAllowed(name, params.URI) {
			continue
		}

		res, err := u.Session.ReadResource(ctx, params)
		if err == nil {
			return res, nil
		}
	}

	return nil, fmt.Errorf("resource not found")
}

func (h *Hub) handleListPrompts(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	var allPrompts []*mcp.Prompt

	for name := range h.Config.Servers {
		u := h.UpstreamManager.Get(name)
		if u == nil || u.Session == nil {
			continue
		}

		result, err := u.Session.ListPrompts(ctx, &mcp.ListPromptsParams{})
		if err != nil {
			continue
		}

		for _, prompt := range result.Prompts {
			if !h.ProfileEngine.IsPromptAllowed(name, prompt.Name) {
				continue
			}
			if h.Config.Hub.PrefixServerIDs {
				p := *prompt
				p.Name = fmt.Sprintf("%s:%s", name, prompt.Name)
				allPrompts = append(allPrompts, &p)
			} else {
				allPrompts = append(allPrompts, prompt)
			}
		}
	}

	return &mcp.ListPromptsResult{
		Prompts: allPrompts,
	}, nil
}

func (h *Hub) handleGetPrompt(ctx context.Context, req mcp.Request) (mcp.Result, error) {
	r, ok := req.(*mcp.GetPromptRequest)
	if !ok {
		return nil, fmt.Errorf("unexpected request type for prompts/get: %T", req)
	}
	params := r.Params

	serverName, promptName, err := h.parseName(params.Name)
	if err != nil {
		return nil, err
	}

	u := h.UpstreamManager.Get(serverName)
	if u == nil || u.Session == nil {
		return nil, fmt.Errorf("server %q not found or not connected", serverName)
	}

	if !h.ProfileEngine.IsPromptAllowed(serverName, promptName) {
		return nil, fmt.Errorf("prompt disabled by policy")
	}

	newParams := *params
	newParams.Name = promptName

	return u.Session.GetPrompt(ctx, &newParams)
}

func (h *Hub) parseName(fullName string) (serverName, localName string, err error) {
	if !h.Config.Hub.PrefixServerIDs {
		return "", "", fmt.Errorf("no-prefix mode not yet implemented")
	}

	parts := strings.SplitN(fullName, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid name format %q, expected server:name", fullName)
	}
	return parts[0], parts[1], nil
}
