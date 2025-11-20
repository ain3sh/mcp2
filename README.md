# mcp2

A Go-based MCP (Model Context Protocol) proxy with profile-based filtering for tools, resources, and prompts.

## Overview

**mcp2** sits between MCP clients (like Claude Code, agents, or custom tools) and multiple upstream MCP servers, providing:

- **Multi-server aggregation**: Connect to multiple upstream MCP servers via stdio or HTTP
- **Profile-based filtering**: Define different profiles (e.g., `safe`, `dev`, `ci`) with fine-grained control over which tools, resources, and prompts are exposed
- **Hub mode**: Aggregate multiple upstream servers into a single MCP endpoint
- **Namespacing**: Optional server ID prefixing to avoid name collisions

## Status

**Phase 1 Complete**: Core infrastructure and basic proxy functionality

- Config loading and validation (YAML/JSON)
- Upstream manager (stdio and HTTP transports)
- Hub server with pass-through aggregation
- CLI commands: `serve`, `validate`
- Comprehensive tests

**Coming Next**:
- Phase 2: Profile-based filtering middleware
- Phase 3: Per-server endpoints and advanced namespacing
- Phase 4: `call` and `profiles` CLI commands
- Phase 5: Integration with f/mcptools

## Installation

```bash
go build -o mcp2 ./cmd/mcp2
```

## Usage

### Validate Configuration

```bash
mcp2 validate -c config.yaml
```

### Run Proxy Server

```bash
# HTTP mode (default)
mcp2 serve -c config.yaml --profile safe --port 8210

# Stdio mode
mcp2 serve -c config.yaml --profile safe --stdio
```

## Configuration

Example configuration file (`config.yaml`):

```yaml
defaultProfile: safe

servers:
  filesystem:
    displayName: "Local Files"
    transport:
      kind: stdio
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/home/user"]
      env:
        NODE_ENV: production

  github:
    displayName: "GitHub"
    transport:
      kind: http
      url: "https://mcp-github.internal/mcp"
      headers:
        Authorization: "Bearer ${GITHUB_TOKEN}"

profiles:
  safe:
    description: "LLM-facing surface; minimal write and delete"
    servers:
      filesystem:
        tools:
          allow: ["list_directory", "read_file"]
          deny: ["write_file", "delete_file"]
        resources:
          allow: ["file://docs/**"]
          deny: ["file://secret/**"]
        prompts: {}

  dev:
    description: "Personal dev; full power"
    servers:
      filesystem:
        tools:
          allow: ["*"]
        resources: {}
        prompts: {}

hub:
  enabled: true
  prefixServerIDs: true

exposePerServer: false
```

### Configuration Schema

**RootConfig**:
- `defaultProfile`: Default profile to use
- `servers`: Map of server ID to server config
- `profiles`: Map of profile name to profile config
- `hub`: Hub configuration
- `exposePerServer`: Whether to expose individual server endpoints

**ServerConfig**:
- `displayName`: Human-readable name
- `transport`: Transport configuration (stdio or http)

**ProfileConfig**:
- `description`: Profile description
- `servers`: Map of server ID to filtering rules

**Filtering Rules** (per profile, per server):
- `tools`: Allow/deny lists for tool names (supports globs)
- `resources`: Allow/deny lists for resource URIs (supports globs)
- `prompts`: Allow/deny lists for prompt names (supports globs)

## Architecture

```
MCP Client (Claude Code, etc.)
    ↓
mcp2 Hub Server (filtered view)
    ↓
Upstream Manager
    ↓
Multiple Upstream MCP Servers (stdio/HTTP)
```

### Components

- **Config Loader**: Parses YAML/JSON configuration
- **Upstream Manager**: Manages connections to upstream servers
- **Hub Server**: Aggregates upstreams into single MCP endpoint
- **Profile Engine** (Phase 2): Enforces filtering policies
- **CLI Layer**: Cobra-based command interface

## Development

### Run Tests

```bash
go test -v ./...
```

### Project Structure

```
mcp2/
├── cmd/mcp2/          # Main entry point and CLI commands
│   ├── main.go
│   └── cmd/
│       ├── root.go
│       ├── serve.go
│       └── validate.go
├── internal/
│   ├── config/        # Configuration loading and validation
│   ├── upstream/      # Upstream server management
│   ├── proxy/         # Hub server implementation
│   └── profile/       # Profile engine (Phase 2)
├── example-config.yaml
└── README.md
```

## License

MIT

## References

- [Model Context Protocol Specification](https://modelcontextprotocol.io/specification/latest)
- [MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk)
- [SPEC.md](./SPEC.md) - Full implementation specification