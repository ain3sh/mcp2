# 1. Overview

**mcp2** is a Go-based MCP proxy + CLI that:

* Sits between MCP clients (Claude Code, agents, your own stuff) and multiple upstream MCP servers.
* Exposes **filtered views** of tools, resources, and prompts based on **profiles** (e.g. `safe`, `dev`, `ci`).
* Enforces filtering at both:

  * **list_*** level (so hidden things never show up)
  * **call_*** level (so guessed / cached names are blocked)
* Uses a **single declarative config file** (YAML or JSON) for:

  * Upstream server definitions
  * Per-profile per-server filters
* Integrates conceptually with **f/mcptools** as the “introspection / discovery” companion CLI.

Core implementation is built on the **official MCP Go SDK**: `github.com/modelcontextprotocol/go-sdk/mcp`.

---

# 2. Goals & Non-Goals

## 2.1 Goals

1. **Multi-server proxy**

   * Support multiple upstream MCP servers wired via stdio or Streamable HTTP transport.

2. **Profiles**

   * Support multiple **profiles** (`safe`, `dev`, `demo`, etc.), each defining different allowed/denied surfaces per server.

3. **Fine-grained filtering**

   * Per-profile, per-server filtering for:

     * Tools (by name)
     * Resources (by URI / template id)
     * Prompts (by name)
   * Hide at `list_*` level and block at `call_*` level.

4. **Single binary, single config**

   * `mcp2` binary, with `mcp2 serve --config config.yaml --profile safe`.
   * Config fully describes upstreams + filters.

5. **CLI usage**

   * CLI subcommands to:

     * Validate config
     * Inspect effective surfaces
     * Call tools / prompts / read resources through the same filtered view the LLM sees.

6. **Play nicely with f/mcptools**

   * Treat **f/mcptools** as the primary “explore / introspect upstreams” tool.
   * mcp2 provides import/merge paths from mcptools outputs into config.

## 2.2 Non-Goals

* Being a full-blown MCP gateway with OAuth, RBAC, metrics dashboards, etc.
* Recreating FastMCP’s decorator sugar in Go.
* Replacing f/mcptools as a general-purpose “call any server” CLI; mcp2’s CLI is for filtered views of upstreams, not for everything under the sun.

---

# 3. Architecture

## 3.1 Core components

1. **Config Loader**

   * Parses a `RootConfig` struct from YAML/JSON.
   * Validates schema, profiles, server definitions.

2. **Profile Engine**

   * Holds active `ProfileConfig` (selected by `--profile`).
   * Provides policy queries:

     * `IsToolAllowed(profile, server, name)`
     * `IsResourceAllowed(profile, server, uri)`
     * `IsPromptAllowed(profile, server, name)`

3. **Upstream Manager**

   * For each configured server:

     * Builds and maintains an `mcp.ClientSession` using go-sdk.
   * Supports:

     * stdio transport: `mcp.NewStdIOTransport()` + `exec.Command` launcher.
     * Streamable HTTP client transport (when landed) or SSE client transport if needed.

4. **Proxy Servers**

   * One **hub** MCP server aggregating all upstreams under the active profile.
   * Optional per-server MCP servers exposed at different HTTP paths.

5. **Filter Middleware**

   * An `mcp.Middleware` that wraps a `MethodHandler` to:

     * Filter `tools/list`, `resources/list`, `prompts/list`.
     * Enforce allow/deny on `tools/call`, `resources/read`, `prompts/get` (and related methods).

6. **CLI Layer**

   * `cobra`-style commands for `serve`, `validate`, `effective`, `call`, etc.
   * Uses `mcp.Client` from the same SDK to talk to mcp2’s HTTP endpoint.

## 3.2 Data flow (hub endpoint)

1. MCP client (e.g. Claude Code) connects to `http://localhost:PORT/mcp` using Streamable HTTP.
2. The **hub server** receives MCP messages, wrapped by filter middleware.
3. For a given method:

   * Proxy layer maps method → appropriate upstream `ClientSession` (based on namespacing / prefixes in config).
   * For `list_*`:

     * Call upstream(s), merge results, apply filtering.
   * For `call_*` / `read_*` / `get_prompt`:

     * Use policy to allow/deny; if allowed, forward to upstream; else return MCP error (e.g. “tool not found” or “disabled by policy”).

## 3.3 Profiles behavior

* On startup, `--profile` selects a `ProfileConfig`.
* Profile config holds per-server `ComponentFilter` for tools/resources/prompts.
* Optional future: profile selected per-request (via header / token), but v1 is “profile for the process.”

---

# 4. Configuration Spec

Use YAML for readability; JSON supported by the same structs.

## 4.1 Schema

Pseudo-Go structs:

```go
type ComponentFilter struct {
    Allow []string `json:"allow" yaml:"allow"` // names or globs
    Deny  []string `json:"deny"  yaml:"deny"`
}

type ServerProfileConfig struct {
    Tools     ComponentFilter `json:"tools"     yaml:"tools"`
    Resources ComponentFilter `json:"resources" yaml:"resources"`
    Prompts   ComponentFilter `json:"prompts"   yaml:"prompts"`
}

type ServerTransportConfig struct {
    // "stdio" or "http"
    Kind string `json:"kind" yaml:"kind"`

    // For stdio
    Command string            `json:"command" yaml:"command"`
    Args    []string          `json:"args"    yaml:"args"`
    Env     map[string]string `json:"env"     yaml:"env"`

    // For HTTP (Streamable HTTP / SSE)
    URL     string            `json:"url"     yaml:"url"`
    Headers map[string]string `json:"headers" yaml:"headers"`
}

type ServerConfig struct {
    DisplayName string               `json:"displayName" yaml:"displayName"`
    Transport   ServerTransportConfig `json:"transport"   yaml:"transport"`
}

type ProfileConfig struct {
    Description string                           `json:"description" yaml:"description"`
    Servers     map[string]ServerProfileConfig `json:"servers"     yaml:"servers"`
}

type RootConfig struct {
    DefaultProfile   string                    `json:"defaultProfile"   yaml:"defaultProfile"`
    Servers          map[string]ServerConfig  `json:"servers"          yaml:"servers"`
    Profiles         map[string]ProfileConfig `json:"profiles"         yaml:"profiles"`

    // Hub behavior
    Hub struct {
        Enabled         bool `json:"enabled" yaml:"enabled"`
        PrefixServerIDs bool `json:"prefixServerIDs" yaml:"prefixServerIDs"`
    } `json:"hub" yaml:"hub"`

    // Optional per-server HTTP endpoints
    ExposePerServer bool `json:"exposePerServer" yaml:"exposePerServer"`
}
```

## 4.2 Example config

```yaml
defaultProfile: safe

servers:
  filesystem:
    displayName: "Local Files"
    transport:
      kind: stdio
      command: npx
      args: ["-y", "@modelcontextprotocol/server-filesystem", "/home/ainesh"]
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
          deny:  ["write_file", "delete_file"]
        resources:
          allow: ["file://docs/**"]
          deny:  ["file://secret/**"]
        prompts:
          deny:  ["dangerous_migrations"]
      github:
        tools:
          allow: ["list_repos", "open_issue"]
          deny:  ["merge_pr"]
        resources: {}
        prompts: {}

  dev:
    description: "Personal dev; full power"
    servers:
      filesystem:
        tools:
          allow: ["*"]
        resources: {}
        prompts: {}
      github:
        tools:
          allow: ["*"]
        resources: {}
        prompts: {}

hub:
  enabled: true
  prefixServerIDs: true

exposePerServer: true
```

Notes:

* `Allow` / `deny` supports globs (`*`, `**`) resolved by `filepath.Match` or similar.
* Behavior:

  * If `allow` empty → “allow all except deny.”
  * If `allow` non-empty → “allow only those, then subtract deny.”

---

# 5. Proxy Behavior (Go SDK)

## 5.1 Startup

1. Parse `--config`, `--profile`, `--port`, `--stdio` flags.

2. Load `RootConfig` from disk (YAML/JSON).

3. Validate:

   * `profile` exists.
   * All referenced servers in profile exist in `servers`.
   * No obviously conflicting rules.

4. Initialize `ProfileEngine` with `RootConfig` + selected profile.

5. For each `ServerConfig`:

   * Build an `Upstream`:

     * For stdio:

       * `exec.Command` + `mcp.NewStdIOTransport()`; connect and create `ClientSession`.
     * For HTTP:

       * Use the HTTP client transport when available, or SSE client transport in the interim as per go-sdk docs.

6. Build MCP servers:

   * **Hub** server if `hub.enabled`:

     * `hubServer := mcp.NewServer(&mcp.Implementation{Name: "mcp2-hub", Version: "0.1.0"}, nil)`
     * Attach filter middleware (see below).
     * Attach handlers that delegate methods to appropriate upstream sessions (tools/resources/prompts).
   * **Per-server** servers if `ExposePerServer`:

     * For each upstream `sid`, create a dedicated `*mcp.Server` exposing only that upstream under profile rules.

7. Attach servers to HTTP:

   * Use `mcp.NewStreamableHTTPHandler(getServer, &mcp.StreamableHTTPOptions{})`.
   * `getServer(req)` returns:

     * `hubServer` for `/mcp`
     * `serverFor("filesystem")` for `/mcp/filesystem`
     * etc.

8. Start HTTP server on `--port`.

Optional: `--stdio` mode where hub server uses `mcp.NewStdIOTransport()` instead, for clients expecting stdio.

## 5.2 Filtering logic

Implement a middleware:

```go
server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
    return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
        srvName := ctx.Value(serverKey{}).(string) // or attach via context

        switch method {

        // LIST PHASE
        case "tools/list":
            res, err := next(ctx, method, req)
            if err != nil { return nil, err }
            lr := res.(*mcp.ListToolsResult)
            lr.Tools = filterToolsForServer(profileEngine, srvName, lr.Tools)
            return lr, nil

        case "resources/list":
            res, err := next(ctx, method, req)
            if err != nil { return nil, err }
            rr := res.(*mcp.ListResourcesResult)
            rr.Resources = filterResourcesForServer(profileEngine, srvName, rr.Resources)
            return rr, nil

        case "prompts/list":
            res, err := next(ctx, method, req)
            if err != nil { return nil, err }
            pr := res.(*mcp.ListPromptsResult)
            pr.Prompts = filterPromptsForServer(profileEngine, srvName, pr.Prompts)
            return pr, nil

        // CALL PHASE
        case "tools/call":
            params := req.GetParams().(*mcp.CallToolParamsRaw)
            if !profileEngine.IsToolAllowed(activeProfile, srvName, params.Name) {
                return nil, mcp.NewError(mcp.ErrMethodNotFound, "tool disabled")
            }
            return next(ctx, method, req)

        case "resources/read":
            params := req.GetParams().(*mcp.ReadResourceParams)
            if !profileEngine.IsResourceAllowed(activeProfile, srvName, params.URI) {
                return nil, mcp.NewError(mcp.ErrMethodNotFound, "resource disabled")
            }
            return next(ctx, method, req)

        case "prompts/get":
            params := req.GetParams().(*mcp.GetPromptParams)
            if !profileEngine.IsPromptAllowed(activeProfile, srvName, params.Name) {
                return nil, mcp.NewError(mcp.ErrMethodNotFound, "prompt disabled")
            }
            return nil, mcp.NewError(mcp.ErrMethodNotFound, "prompt disabled")
        }

        return next(ctx, method, req)
    }
})
```

Key points:

* **List-phase filtering** ensures hidden components do not appear to clients (prevents context pollution).
* **Call-phase checks** guarantee that even if a client guesses or re-uses a cached tool/resource/prompt name, the call is blocked.

## 5.3 Name handling in hub

Two options; configurable via `hub.prefixServerIDs`:

1. **Prefix mode (`true`)**

   * Tools are exposed as `filesystem:list_directory`, `github:list_repos`, etc.
   * Proxy maps prefix back to upstream server id.
   * Avoids collisions and ambiguity.

2. **No prefix (`false`)**

   * If multiple upstreams define the same tool name:

     * Either error at startup (config validation) or pick a deterministic winner.

Prefix mode is safer and more explicit; default should be `true`.

---

# 6. CLI Design (`mcp2`)

## 6.1 Top-level commands

Suggested:

* `mcp2 serve`
  Run proxy as MCP server.

* `mcp2 validate`
  Validate config file.

* `mcp2 effective`
  Show effective tools/resources/prompts for a given profile & server.

* `mcp2 call`
  Call a tool or prompt through the filtered view (against hub or specific server).

* `mcp2 profiles`
  List available profiles and their descriptions.

### 6.2 `mcp2 serve`

Flags:

* `--config path` (default: `~/.config/mcp2/config.yaml`)
* `--profile name` (default: from config)
* `--port N` (default: `8210`)
* `--stdio` (optional; if set, run hub server on stdio instead of HTTP)

Usage:

```bash
mcp2 serve --config ~/.config/mcp2/config.yaml --profile safe --port 8210
```

Client then points at:

* HTTP (Streamable HTTP): `http://127.0.0.1:8210/mcp`

## 6.3 `mcp2 validate`

Loads config and:

* Ensures all servers referenced in profiles exist.
* Ensures `defaultProfile` exists.
* Detects name collisions if `prefixServerIDs` is false.

Exit code non-zero on errors.

## 6.4 `mcp2 effective`

Example:

```bash
mcp2 effective --config config.yaml --profile safe --server filesystem
```

Outputs:

* Allowed tools
* Denied tools
* Allowed / denied resource patterns
* Allowed / denied prompts

This is for your sanity check before letting the LLM near it.

## 6.5 `mcp2 call`

This uses go-sdk’s client to talk to **mcp2 itself**, not directly to upstreams, so behavior matches LLM-facing surface.

Example:

```bash
mcp2 call \
  --port 8210 \
  tool \
  --name filesystem:list_directory \
  --params '{"path":"/home/ainesh/projects"}'
```

Or for prompts:

```bash
mcp2 call \
  prompt \
  --name github:open_issue_template \
  --args '{"repo":"ain3sh/servers"}'
```

This gives you a CLI “agent” with the same restrictions as the profile given to Claude or other hosts.

---

# 7. Integration with f/mcptools

f/mcptools is already a fluent CLI for discovering & calling tools / prompts over HTTP or stdio.

We integrate at the **workflow** level, not by linking against it.

## 7.1 Use cases for mcptools in this stack

1. **Discover capabilities of upstream servers**

   * `mcptools` (or `mcpt`) can:

     * Connect to an MCP server.
     * List tools, resources, prompts as JSON.

2. **Bootstrap mcp2 config**

   * You can write `mcp2 init` (optional) that:

     * Reads a minimal upstream list (maybe from a seed file or env).
     * Shells out to `mcptools` to fetch inventories.
     * Generates a bare-bones `config.yaml` with:

       * One `servers` entry per upstream.
       * Profiles:

         * `safe` with conservative defaults.
         * `dev` allowing all.

Alternatively: you use mcptools directly and run a separate script that merges its output into `config.yaml`.

## 7.2 mcptools flows

Example:

```bash
# Use mcptools to inspect upstream filesystem server
mcpt http http://127.0.0.1:9000/mcp tools list --json > inventory/filesystem-tools.json
mcpt http http://127.0.0.1:9000/mcp prompts list --json > inventory/filesystem-prompts.json
mcpt http http://127.0.0.1:9000/mcp resources list --json > inventory/filesystem-resources.json
```

Then your (small) `mcp2 init` command can:

* Parse these inventories.
* Propose default allow lists for `safe` and `dev` profiles.
* Write/update `config.yaml` accordingly.

This keeps **mcp2** focused on policy/proxy, and **mcptools** on exploration. No duplicated functionality.

---

# 8. Security & Safety Considerations

* **Filtering is not security** unless enforced at both list and call time.
  This spec explicitly does that.

* **Upstream trust**
  mcp2 assumes upstream servers are not hostile and won’t go out of their way to bypass policy once called.

* **Resource URIs**

  * Allow/deny patterns for resources should be globbed carefully.
  * Example: `file://secret/**` must actually block secrets directories; test this.

* **Config hygiene**

  * Config should be version-controlled.
  * Consider a `mcp2 validate` in CI as a simple gate.

* **Transport**

  * Local-only by default (`127.0.0.1`), TLS optional later if you expose mcp2 remotely.

---

# 9. Implementation Phases

## Phase 1: Skeleton

* Wire Go module using `github.com/modelcontextprotocol/go-sdk/mcp`.
* Implement:

  * Config loader (no profiles yet; single flat filters).
  * Upstream Manager (basic stdio and HTTP).
  * Single hub server with pass-through (no filtering).
  * `mcp2 serve` and `mcp2 validate`.

Goal: working, dumb proxy.

## Phase 2: Profiles & Filtering

* Introduce `profiles` structure.
* Implement `ProfileEngine`.
* Add middleware that:

  * Filters `list_*` based on profile.
  * Blocks `call_*` for disabled components.
* Add `mcp2 effective` command.

Goal: fully working profile-aware filter.

## Phase 3: Aggregation & Namespacing

* Implement:

  * Hub mode with optional `prefixServerIDs`.
  * Per-server endpoints: `/mcp/<server>` via `NewStreamableHTTPHandler(getServer, ...)`.
* Validate name-collision handling.

Goal: one hub endpoint + per-server endpoints, under policies.

## Phase 4: CLI `call` & `profiles`

* Implement `mcp2 call`:

  * Use Go SDK client piece to talk to hub endpoint and issue tool/prompt/resource calls via HTTP.
* Implement `mcp2 profiles` to list available profiles from config.

Goal: you can use the exact same filtered surface in CLI mode.

## Phase 5: mcptools integration helpers

* Add optional `mcp2 init` or `mcp2 import-inventory` that:

  * Reads JSON from mcptools listing output.
  * Generates/updates config sections for servers + profiles.

Goal: reduce manual editing when adding new upstreams.

## Phase 6 (Optional / Future): TUI & per-request profiles

* TUI for editing filters:

  * Use `tcell` / `bubbletea` / `textual`-style Go TUI.
  * Show profiles → servers → tools/resources/prompts with checkboxes.
* Per-request profile selection:

  * Optionally inspect headers (e.g., `X-MCP2-Profile`) to select profile for each MCP session.

This is bonus candy, not required for initial “works and doesn’t expose delete_prod_db to Claude” milestone.

---

# 10. References & Resources

A few things worth bookmarking and not pretending you’ll “remember later”:

* **Official Go SDK for MCP**
  `github.com/modelcontextprotocol/go-sdk`

* **MCP SDK overview (multi-language)**
  General description of SDK capabilities and parity.

* **MCP spec: tools, resources, prompts**

  * Tools & interaction overview
  * Resources spec
  * Prompts spec

* **Streamable HTTP transport**

  * Spec & rationale

* **Go SDK design & HTTP handler pattern**
  Especially how `NewSSEHTTPHandler` / Streamable HTTP handlers take a `getServer` callback for per-session/per-path servers.

* **f/mcptools** (CLI for MCP servers)
  `github.com/f/mcptools` and its `client` package docs.
