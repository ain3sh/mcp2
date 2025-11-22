# f/mcptools vs mcp2 - Complete Analysis

## Relationship Status

**NO INTEGRATION, NO DEPENDENCY, NO BORROWED CODE**

- mcp2 does **not** import f/mcptools as a dependency
- mcp2 does **not** use any code from f/mcptools
- Both are independent projects solving **complementary** problems
- Both use **different MCP SDKs**:
  - **mcptools**: Uses `github.com/mark3labs/mcp-go` (community SDK)
  - **mcp2**: Uses `github.com/modelcontextprotocol/go-sdk/mcp` (official SDK)

---

## What f/mcptools IS

**Primary Purpose:** MCP **CLIENT** toolkit and Swiss Army knife

### Core Functions (as CLIENT)

1. **Connect TO MCP servers** (we connect clients TO us)
2. **List tools/resources/prompts** from servers
3. **Call tools/get prompts/read resources** directly
4. **Interactive shell** for exploring MCP servers
5. **Web UI** for browsing MCP servers

### Server Modes (specialty features)

1. **Mock Server** - Creates fake MCP servers for testing clients
2. **Proxy Server** - Wraps shell scripts as MCP tools
3. **Guard Mode** - **Filters MCP servers** (SIMILAR TO mcp2!)

### Additional Features

- **Server aliases** - Save/reuse server commands (`~/.mcpt/aliases.json`)
- **Config management** - Manage MCP configs for VS Code, Cursor, Claude Desktop, etc.
- **Project scaffolding** - Generate new MCP server projects (TypeScript)
- **Output formats** - Table, JSON, pretty JSON

### Guard Mode (Their Filtering Feature)

**How it works:**
```bash
# Allow only read operations
mcp guard --allow 'tools:read_*' --deny 'tools:write_*' npx -y @modelcontextprotocol/server-filesystem ~
```

**Implementation:**
- Runs as **stdio proxy** (intercepts JSON-RPC on stdin/stdout)
- Filters `tools/list`, `prompts/list`, `resources/list` responses
- Blocks `tools/call`, `prompts/get`, `resources/read` requests
- Uses `filepath.Match` for glob patterns
- Logs to `~/.mcpt/logs/guard.log`
- **LIMITATION:** Only stdio transport, not HTTP

---

## What mcp2 IS

**Primary Purpose:** MCP **PROXY/GATEWAY** with multi-server aggregation and profile-based filtering

### Core Functions

1. **Sits BETWEEN MCP clients and servers** (we're the middleman)
2. **Aggregates multiple upstream servers** into one hub endpoint
3. **Profile-based filtering** (safe, dev, ci profiles)
4. **Per-server isolated endpoints** (`/mcp/{serverID}`)
5. **Two-phase filtering** (list + call phase) for security

### Key Differentiators vs mcptools

1. **Multi-server aggregation** - mcptools connects to ONE server at a time
2. **HTTP + stdio support** - We serve HTTP endpoints, not just stdio
3. **Profile system** - Multiple named filter configurations
4. **Server namespacing** - Prefix tools with `serverID:toolname` to avoid collisions
5. **Persistent gateway** - Runs as long-lived HTTP service
6. **Official MCP SDK** - Uses official Go SDK from modelcontextprotocol

---

## Feature Comparison Matrix

| Feature | mcptools | mcp2 | Winner |
|---------|----------|------|--------|
| **Core Purpose** | MCP Client | MCP Proxy/Gateway | Different roles |
| **Connect to MCP servers** | ✅ Yes (as client) | ✅ Yes (as proxy) | Both |
| **List tools/resources/prompts** | ✅ Yes | ✅ Yes | Both |
| **Call tools/prompts/resources** | ✅ Yes | ✅ Yes | Both |
| **Filter/restrict MCP servers** | ✅ Yes (guard mode) | ✅ Yes (profiles) | Both |
| **Multi-server aggregation** | ❌ No | ✅ Yes | **mcp2** |
| **HTTP endpoint serving** | ⚠️ Web UI only | ✅ Yes (full MCP) | **mcp2** |
| **Profile management** | ❌ No | ✅ Yes | **mcp2** |
| **Server namespacing** | ❌ No | ✅ Yes | **mcp2** |
| **Per-server isolation** | ❌ No | ✅ Yes | **mcp2** |
| **Interactive shell** | ✅ Yes | ❌ No | **mcptools** |
| **Web UI** | ✅ Yes | ❌ No | **mcptools** |
| **Mock server** | ✅ Yes | ❌ No | **mcptools** |
| **Proxy shell scripts** | ✅ Yes | ❌ No | **mcptools** |
| **Server aliases** | ✅ Yes | ❌ No | **mcptools** |
| **Config management** | ✅ Yes (VS Code, etc.) | ❌ No | **mcptools** |
| **Project scaffolding** | ✅ Yes | ❌ No | **mcptools** |
| **Output formats** | ✅ Table/JSON/pretty | ⚠️ JSON only | **mcptools** |
| **Transports supported** | stdio, HTTP, SSE | stdio, HTTP | **mcptools** |
| **HTTP for filtering** | ❌ No (stdio only) | ✅ Yes | **mcp2** |
| **Long-lived service** | ⚠️ Proxy mode only | ✅ Yes | **mcp2** |
| **Official MCP SDK** | ❌ No (mark3labs) | ✅ Yes | **mcp2** |

---

## Overlap: Filtering Features

Both implement MCP filtering, but **very differently**:

### mcptools Guard Mode
- **Use case:** Wrap a single server to restrict it for one-time use
- **Implementation:** stdio proxy (JSON-RPC interceptor)
- **Configuration:** CLI flags `--allow`/`--deny`
- **Persistence:** No config file, flags each time
- **Multi-server:** No, one server at a time
- **Transport:** stdio only
- **Logging:** `~/.mcpt/logs/guard.log`

### mcp2 Profiles
- **Use case:** Persistent multi-server gateway with named profiles
- **Implementation:** HTTP + stdio MCP server with middleware
- **Configuration:** YAML config file with multiple profiles
- **Persistence:** Config file, reload = restart
- **Multi-server:** Yes, aggregates multiple servers
- **Transport:** HTTP and stdio
- **Logging:** Console output (no file logging yet)

**Example comparison:**

**mcptools approach:**
```bash
# One-off command
mcp guard --allow 'tools:read_*' --deny 'tools:write_*' npx -y @modelcontextprotocol/server-filesystem ~
```

**mcp2 approach:**
```yaml
# config.yaml - persistent, multi-profile, multi-server
profiles:
  safe:
    servers:
      filesystem:
        tools:
          allow: ["read_*"]
          deny: ["write_*"]
      github:
        tools:
          allow: ["list_repos"]
```

```bash
# Start persistent HTTP gateway
mcp2 serve -c config.yaml -p safe --port 8210
```

---

## Use Case Analysis

### When to use mcptools

1. **Exploring/debugging MCP servers** - Interactive shell, web UI
2. **Quick tool calls** - One-off commands to MCP servers
3. **Testing MCP clients** - Mock server mode
4. **Wrapping shell scripts** - Make scripts available as MCP tools
5. **Managing app configs** - Add servers to VS Code, Cursor, etc.
6. **Project creation** - Scaffold new MCP servers
7. **One-time filtering** - Quick guard wrapper for demo/test

### When to use mcp2

1. **Production MCP gateway** - Long-lived HTTP service
2. **Multi-server aggregation** - Combine multiple MCP servers
3. **Profile-based access control** - Different filters for different clients/contexts
4. **Server namespacing** - Avoid tool name collisions across servers
5. **Per-server isolation** - Expose individual servers at separate endpoints
6. **Enterprise filtering** - Persistent, auditable filter configs
7. **LLM-facing proxy** - Restrict what Claude/GPT can access

---

## Integration Opportunities

### How they COULD work together

1. **mcptools → mcp2**
   - Use `mcpt configs set` to configure apps to connect to mcp2 gateway
   - Example: Point Claude Desktop to `http://localhost:8210/mcp` instead of individual servers

2. **mcp2 leveraging mcptools concepts**
   - Add interactive shell (like mcpt shell)
   - Add web UI for config management
   - Add server aliases
   - Import mcptools' `configs scan` output to generate mcp2 config

3. **mcptools using mcp2**
   - `mcpt tools http://localhost:8210/mcp` - Explore aggregated servers through mcp2
   - `mcpt guard` could point to mcp2 instead of upstream servers

---

## Summary Matrix (Updated with mcptools context)

| Requirement | mcptools | mcp2 Status | Notes |
|-------------|----------|-------------|-------|
| **Connect to servers** | ✅ Client | ✅ Proxy | Different roles, both work |
| **HTTP/stdio/SSE** | ✅ All 3 | ⚠️ HTTP, stdio | mcptools has SSE, we don't |
| **Authentication** | ❌ No | ❌ No | Neither supports auth yet |
| **View descriptions** | ✅ Yes | ✅ Yes | Both show full descriptions |
| **Toggle availability** | ✅ guard flags | ✅ profiles | Different UX, same result |
| **Arbitrary groupings** | ❌ No | ⚠️ Partial | mcp2 profiles, but no custom server names |
| **Custom endpoints** | ❌ No | ✅ Yes | mcp2 per-server endpoints |
| **Multi-profile** | ❌ No | ⚠️ Restart | mcp2 has profiles but needs restart |
| **True obfuscation** | ✅ Yes | ✅ Yes | Both filter list + block calls |
| **Long-lived calls** | ✅ Yes | ✅ Yes | Both support passthrough |
| **Performance** | ✅ Direct | ✅ Good | Both minimal overhead |
| **Port in config** | N/A | ❌ No | CLI flag only |
| **TUI** | ✅ Shell | ❌ No | mcptools has interactive shell |
| **Web UI** | ✅ Yes | ❌ No | mcptools has web interface |
| **No-auth** | ✅ Yes | ✅ Yes | Both currently no-auth |

---

## Recommendation: Complementary Tools Strategy

**DON'T rebuild what mcptools does well:**
- ❌ Don't add interactive shell (use `mcpt shell`)
- ❌ Don't add web UI (use `mcpt web`)
- ❌ Don't add project scaffolding (use `mcpt new`)
- ❌ Don't add config scanning (use `mcpt configs scan`)

**DO focus on what mcp2 does uniquely:**
- ✅ Multi-server aggregation
- ✅ Profile-based filtering
- ✅ HTTP gateway serving
- ✅ Per-server isolation
- ✅ Enterprise-grade config management
- ✅ Server namespacing

**DO add integrations:**
- ✅ Import from mcptools aliases
- ✅ Generate mcptools-compatible config
- ✅ Document how to use mcptools with mcp2
- ✅ Add command: `mcp2 import-mcptools-aliases`

---

## Gaps to Address (Updated)

### Critical for Production
1. **Upstream authentication** - Headers/OAuth not wired (mcptools lacks this too)
2. **Hot config reload** - Currently requires restart
3. **Port in config file** - Currently CLI flag only

### Nice to Have
4. **SSE transport** - mcptools has it, we don't
5. **Web UI integration** - Link to/embed mcptools web UI?
6. **Import mcptools aliases** - `mcp2 import ~/.mcpt/aliases.json`
7. **Custom virtual server names** - Expose friendly names to MCP clients

### Lower Priority
8. **TUI for config** - mcptools doesn't have this either
9. **Logging to file** - mcptools logs to `~/.mcpt/logs/`, we don't

---

## Conclusion

**f/mcptools and mcp2 are COMPLEMENTARY, not competitive:**

- **mcptools** = Swiss Army knife for MCP developers/users
- **mcp2** = Enterprise MCP gateway for production deployments

**Perfect workflow:**
1. Use `mcpt` to explore/test MCP servers
2. Configure `mcp2` to aggregate servers for production
3. Point LLM clients to mcp2 gateway
4. Use `mcpt configs` to manage client configurations
5. Use `mcpt shell` to debug issues through mcp2 proxy

**They solve different problems and should work together, not replace each other.**
