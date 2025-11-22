# mcp2

A Go-based MCP (Model Context Protocol) proxy with profile-based filtering for tools, resources, and prompts.

## Overview

**mcp2** sits between MCP clients (like Claude Code, agents, or custom tools) and multiple upstream MCP servers, providing:

- **Multi-server aggregation**: Connect to multiple upstream MCP servers via stdio or HTTP
- **Profile-based filtering**: Define different profiles (e.g., `safe`, `dev`, `ci`) with fine-grained control over which tools, resources, and prompts are exposed
- **Hub mode**: Aggregate multiple upstream servers into a single MCP endpoint
- **Namespacing**: Optional server ID prefixing to avoid name collisions

## Status

**Phase 4 Complete**: CLI `call` & `profiles` Commands

- ✅ `mcp2 profiles` - List available profiles with descriptions and filter counts
- ✅ `mcp2 call tool` - Call tools through the filtered view
- ✅ `mcp2 call prompt` - Get prompts through the filtered view
- ✅ `mcp2 call resource` - Read resources through the filtered view
- ✅ JSON output support (`--json` flag)
- ✅ Hub and per-server endpoint support (`--endpoint` flag)
- ✅ Timeout configuration (`--timeout` flag)
- ✅ Uses same filtering rules as LLM-facing surface

**Previous Phases**:
- Phase 3: Per-server endpoints & HTTP routing (integration tested with Context7)
- Phase 2: Profile-based filtering (ProfileEngine, glob matching, list/call-phase filtering)
- Phase 1: Core infrastructure (config, upstream manager, hub server)

**Coming Next**:
- Phase 5: Integration with f/mcptools (init/import-inventory commands)

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

### Inspect Effective Filtering Rules

```bash
# Show what tools/resources/prompts are allowed for a server in a profile
mcp2 effective -c config.yaml -p safe -s filesystem
```

### List Available Profiles

```bash
# Display all profiles with descriptions and filter information
mcp2 profiles -c config.yaml
```

### Call Tools/Prompts/Resources Through Filtered View

The `call` command lets you interact with MCP servers through the same filtered view that LLMs see:

```bash
# Call a tool through the hub endpoint (with server prefix)
mcp2 call tool --name context7:get-library-docs \
  --params '{"context7CompatibleLibraryID":"/websites/react_dev"}' \
  --port 8210

# Call a tool through per-server endpoint (no prefix needed)
mcp2 call tool --name get-library-docs \
  --params '{"context7CompatibleLibraryID":"/websites/react_dev"}' \
  --port 8210 --endpoint /mcp/context7

# Get a prompt
mcp2 call prompt --name github:issue_template \
  --args '{"repo":"ain3sh/mcp2"}' \
  --port 8210

# Read a resource
mcp2 call resource --uri file:///home/user/README.md \
  --port 8210

# Get JSON output (for programmatic use)
mcp2 call tool --name context7:resolve-library-id \
  --params '{"libraryName":"react"}' \
  --port 8210 --json

# Set custom timeout (default: 30 seconds)
mcp2 call tool --name slow-operation \
  --params '{}' \
  --port 8210 --timeout 60
```

### Access Per-Server Endpoints

When `exposePerServer: true` in your config:

```bash
# Start server with per-server endpoints
mcp2 serve -c config.yaml -p safe --port 8210

# Endpoints:
# http://localhost:8210/mcp              - Hub (aggregates all servers with prefixing)
# http://localhost:8210/mcp/filesystem   - Direct access to filesystem server only
# http://localhost:8210/mcp/github       - Direct access to github server only
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
- **Hub Server**: Aggregates upstreams into single MCP endpoint with prefixing
- **Per-Server Proxies** (Phase 3): Individual filtered endpoints per upstream
- **Profile Engine** (Phase 2): Enforces filtering policies
- **CLI Layer**: Cobra-based command interface

### HTTP Routing (Phase 3)

```
Client Request
     ↓
HTTP Router (ServeMux)
     ├─→ /mcp              → Hub Server (aggregated, prefixed)
     ├─→ /mcp/filesystem   → Filesystem Proxy (isolated, no prefix)
     └─→ /mcp/github       → GitHub Proxy (isolated, no prefix)
```

Each endpoint enforces the same profile-based filtering independently.

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