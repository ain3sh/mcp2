# mcp2 Remaining Work - Production Readiness Spec

**Status**: Phase 4 complete. mcp2 is a functional MCP gateway with multi-server aggregation and profile-based filtering.

**Goal**: Make mcp2 production-ready as the networking powerhouse for MCP infrastructure.

**Context**: With mcptools handling exploration/debugging (interactive shell, web UI, mock servers), mcp2 focuses on being the enterprise-grade gateway/proxy for production deployments.

---

## Critical Path to Production (P0)

### 1. Upstream Server Authentication

**Problem**: Headers config exists but is never passed to HTTP transports. Cannot authenticate to upstream MCP servers.

**Current State**:
```go
// internal/config/types.go:29
Headers map[string]string `json:"headers" yaml:"headers"`

// internal/upstream/manager.go:146-149
return &mcp.StreamableClientTransport{
    Endpoint: serverCfg.Transport.URL,
    // TODO: Add support for custom headers via middleware or transport options
}, nil
```

**Required Changes**:

1. Wire up headers to `StreamableClientTransport`:
```go
// manager.go:144-150
func createHTTPTransport(serverCfg *config.ServerConfig) (mcp.Transport, error) {
    // Create custom HTTP client with headers
    httpClient := &http.Client{
        Timeout: 30 * time.Second,
    }

    // Create transport with custom client
    transport := &mcp.StreamableClientTransport{
        Endpoint:   serverCfg.Transport.URL,
        HTTPClient: httpClient,
        MaxRetries: 5,
    }

    // Wrap with header injection middleware if headers exist
    if len(serverCfg.Transport.Headers) > 0 {
        // Need to use custom RoundTripper
        httpClient.Transport = &headerInjector{
            base:    http.DefaultTransport,
            headers: serverCfg.Transport.Headers,
        }
    }

    return transport, nil
}

type headerInjector struct {
    base    http.RoundTripper
    headers map[string]string
}

func (h *headerInjector) RoundTrip(req *http.Request) (*http.Response, error) {
    for k, v := range h.headers {
        req.Header.Set(k, v)
    }
    return h.base.RoundTrip(req)
}
```

2. Support common auth patterns in examples:
```yaml
servers:
  github:
    transport:
      kind: http
      url: "https://api.github.com/mcp"
      headers:
        Authorization: "Bearer ${GITHUB_TOKEN}"  # Env var expansion already works
        X-API-Key: "${API_KEY}"

  authenticated-service:
    transport:
      kind: http
      url: "https://service.example.com/mcp"
      headers:
        Authorization: "Bearer ${SERVICE_TOKEN}"
        X-Custom-Header: "custom-value"
```

**Files to Modify**:
- `internal/upstream/manager.go` - Implement `headerInjector` and wire to transport
- `example-config.yaml` - Add authenticated server example
- `README.md` - Document authentication patterns

**Acceptance Criteria**:
- Can connect to HTTP MCP servers requiring Bearer tokens
- Can connect to servers requiring custom headers
- Environment variable expansion works in header values
- Integration test with authenticated endpoint

---

### 2. SSE Transport Support

**Problem**: mcptools supports SSE, we don't. Some MCP servers only expose SSE endpoints (legacy 2024-11-05 protocol).

**Current State**:
```go
// manager.go:55-61
switch serverCfg.Transport.Kind {
case "stdio":
    transport, err = createStdioTransport(serverCfg)
case "http":
    transport, err = createHTTPTransport(serverCfg)
default:
    return fmt.Errorf("unsupported transport kind: %q", serverCfg.Transport.Kind)
}
```

**Required Changes**:

1. Add SSE transport kind to config:
```go
// config/types.go - Update documentation
type ServerTransportConfig struct {
    // Kind is "stdio", "http", or "sse"
    Kind string `json:"kind" yaml:"kind"`

    // For stdio transport
    Command string            `json:"command" yaml:"command"`
    Args    []string          `json:"args" yaml:"args"`
    Env     map[string]string `json:"env" yaml:"env"`

    // For HTTP/SSE transport
    URL     string            `json:"url" yaml:"url"`
    Headers map[string]string `json:"headers" yaml:"headers"`
}
```

2. Implement SSE transport creator:
```go
// manager.go - Add SSE support
case "sse":
    transport, err = createSSETransport(serverCfg)

// Add new function
func createSSETransport(serverCfg *config.ServerConfig) (mcp.Transport, error) {
    httpClient := &http.Client{
        Timeout: 0, // SSE needs long-lived connections
    }

    // Add header injection if needed
    if len(serverCfg.Transport.Headers) > 0 {
        httpClient.Transport = &headerInjector{
            base:    http.DefaultTransport,
            headers: serverCfg.Transport.Headers,
        }
    }

    return &mcp.SSEClientTransport{
        Endpoint:   serverCfg.Transport.URL,
        HTTPClient: httpClient,
    }, nil
}
```

3. Update config validation:
```go
// config/validator.go
func validateServerConfig(serverID string, server *ServerConfig) error {
    // Validate transport kind
    switch server.Transport.Kind {
    case "stdio", "http", "sse":
        // Valid
    default:
        return fmt.Errorf("server %q: invalid transport kind %q (must be stdio, http, or sse)",
            serverID, server.Transport.Kind)
    }

    // ... rest of validation
}
```

**Files to Modify**:
- `internal/config/types.go` - Update docs
- `internal/config/validator.go` - Add "sse" to valid kinds
- `internal/upstream/manager.go` - Add SSE transport support
- `example-config.yaml` - Add SSE server example
- `README.md` - Document SSE transport

**Acceptance Criteria**:
- Can connect to SSE MCP servers
- Headers work with SSE transport
- Integration test with SSE endpoint (use mcptools mock server or public SSE endpoint)

---

### 3. Port and Host in Config File

**Problem**: Port is CLI flag only. Can't set via config file. No way to configure bind address.

**Current State**:
```go
// cmd/mcp2/cmd/serve.go:34
serveCmd.Flags().IntVarP(&port, "port", "", 8210, "port to listen on")

// serve.go:98
addr := fmt.Sprintf("127.0.0.1:%d", port)
```

**Required Changes**:

1. Add server config to RootConfig:
```go
// config/types.go
type ServerListenConfig struct {
    Host string `json:"host" yaml:"host"` // Default: "127.0.0.1"
    Port int    `json:"port" yaml:"port"` // Default: 8210
}

type RootConfig struct {
    DefaultProfile  string                   `json:"defaultProfile" yaml:"defaultProfile"`
    Servers         map[string]ServerConfig  `json:"servers" yaml:"servers"`
    Profiles        map[string]ProfileConfig `json:"profiles" yaml:"profiles"`
    Hub             HubConfig                `json:"hub" yaml:"hub"`
    ExposePerServer bool                     `json:"exposePerServer" yaml:"exposePerServer"`

    // NEW: Server listen configuration
    Listen          ServerListenConfig       `json:"listen" yaml:"listen"`
}
```

2. Update serve command to use config with CLI override:
```go
// cmd/mcp2/cmd/serve.go
func runServe(cmd *cobra.Command, args []string) error {
    // ... load config ...

    // Determine listen address
    listenHost := cfg.Listen.Host
    listenPort := cfg.Listen.Port

    // CLI flags override config
    if cmd.Flags().Changed("host") {
        listenHost = hostFlag // Need to add this flag
    }
    if cmd.Flags().Changed("port") {
        listenPort = port
    }

    // Apply defaults if not set
    if listenHost == "" {
        listenHost = "127.0.0.1"
    }
    if listenPort == 0 {
        listenPort = 8210
    }

    addr := fmt.Sprintf("%s:%d", listenHost, listenPort)

    // ... rest of function ...
}
```

3. Add host flag:
```go
// cmd/mcp2/cmd/serve.go:32-35
var (
    port  int
    host  string
    stdio bool
)

func init() {
    rootCmd.AddCommand(serveCmd)
    serveCmd.Flags().StringVarP(&host, "host", "", "", "host to bind to (default from config or 127.0.0.1)")
    serveCmd.Flags().IntVarP(&port, "port", "", 0, "port to listen on (default from config or 8210)")
    serveCmd.Flags().BoolVarP(&stdio, "stdio", "", false, "use stdio transport instead of HTTP")
}
```

4. Update example configs:
```yaml
# example-config.yaml
defaultProfile: safe

# Server listen configuration
listen:
  host: "127.0.0.1"  # localhost only
  port: 8210

# Or for all interfaces:
# listen:
#   host: "0.0.0.0"
#   port: 8210

servers:
  # ... rest of config
```

**Files to Modify**:
- `internal/config/types.go` - Add `ServerListenConfig` and `Listen` field
- `cmd/mcp2/cmd/serve.go` - Add host flag, use config values with CLI override
- `example-config.yaml` - Add listen config
- `example-config-perserver.yaml` - Add listen config
- `test-context7.yaml` - Add listen config
- `README.md` - Document listen configuration

**Acceptance Criteria**:
- Port configurable in YAML
- Host configurable in YAML
- CLI flags override config values
- Defaults work when not specified (127.0.0.1:8210)
- Can bind to 0.0.0.0 for remote access

---

## High Priority (P1)

### 4. Hot Config Reload

**Problem**: Must restart server to change profiles or filters. Downtime for config changes.

**Implementation Options**:

**Option A: File watching (Recommended)**
```go
// Add to serve.go
import "github.com/fsnotify/fsnotify"

func watchConfig(configPath string, reloadFunc func()) error {
    watcher, err := fsnotify.NewWatcher()
    if err != nil {
        return err
    }
    defer watcher.Close()

    if err := watcher.Add(configPath); err != nil {
        return err
    }

    for {
        select {
        case event := <-watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write {
                log.Println("Config file changed, reloading...")
                reloadFunc()
            }
        case err := <-watcher.Errors:
            log.Printf("Watcher error: %v", err)
        }
    }
}
```

**Option B: Signal handling (Alternative)**
```go
// Reload on SIGHUP
signal.Notify(sighup, syscall.SIGHUP)
go func() {
    for range sighup {
        log.Println("Received SIGHUP, reloading config...")
        reloadConfig()
    }
}()
```

**Option C: HTTP endpoint (Bonus)**
```go
// Add admin endpoint
mux.HandleFunc("/_mcp2/reload", func(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    log.Println("Reload requested via HTTP endpoint")
    reloadConfig()
    w.WriteHeader(http.StatusOK)
    fmt.Fprintln(w, "Config reloaded")
})
```

**Reload Logic**:
```go
func reloadConfig() error {
    // Load new config
    newCfg, err := config.Load(configPath)
    if err != nil {
        log.Printf("Failed to load new config: %v", err)
        return err
    }

    newCfg.ExpandEnvVars()

    if err := newCfg.Validate(); err != nil {
        log.Printf("Invalid new config: %v", err)
        return err
    }

    // Determine if we need to reconnect to upstreams
    // (server configs changed, new servers added, etc.)
    needReconnect := serversChanged(cfg, newCfg)

    if needReconnect {
        // Close old connections
        manager.Close()

        // Create new manager and reconnect
        newManager := upstream.NewManager()
        for serverID, serverCfg := range newCfg.Servers {
            if err := newManager.Connect(ctx, serverID, &serverCfg); err != nil {
                log.Printf("Failed to reconnect to %s: %v", serverID, err)
                // Rollback? Or continue with partial?
            }
        }
        manager = newManager
    }

    // Recreate hub with new profile
    hub = proxy.NewHub(newCfg, manager, activeProfile)

    // Update global config
    cfg = newCfg

    log.Println("Config reloaded successfully")
    return nil
}
```

**Files to Modify**:
- `go.mod` - Add `github.com/fsnotify/fsnotify`
- `cmd/mcp2/cmd/serve.go` - Add config watching, reload logic
- `README.md` - Document reload mechanisms

**Acceptance Criteria**:
- Config file changes trigger reload
- Profiles update without restart
- Filter changes apply immediately
- Server connections re-established if needed
- Errors in new config don't break running server (rollback)
- SIGHUP triggers reload
- Optional HTTP endpoint for programmatic reload

**Complexity**: Medium-High. Need to handle connection lifecycle carefully.

---

### 5. Structured Logging to File

**Problem**: All logs go to stderr. No file logging, no log rotation, no structured logs.

**Current State**:
```go
log.Printf("Loading config from: %s", path)
log.Printf("Connected to %s via %s transport", serverID, serverCfg.Transport.Kind)
```

**Required Changes**:

1. Add logging config to RootConfig:
```go
// config/types.go
type LoggingConfig struct {
    Level      string `json:"level" yaml:"level"`           // debug, info, warn, error
    File       string `json:"file" yaml:"file"`             // Log file path, empty = stderr only
    MaxSizeMB  int    `json:"maxSizeMB" yaml:"maxSizeMB"`   // Max size before rotation
    MaxBackups int    `json:"maxBackups" yaml:"maxBackups"` // Number of old logs to keep
    MaxAgeDays int    `json:"maxAgeDays" yaml:"maxAgeDays"` // Max days to keep logs
    Compress   bool   `json:"compress" yaml:"compress"`     // Compress rotated logs
}

type RootConfig struct {
    // ... existing fields ...
    Logging LoggingConfig `json:"logging" yaml:"logging"`
}
```

2. Use a proper logging library (options):
   - **Option A**: `log/slog` (stdlib, Go 1.21+) - Simple, no dependencies
   - **Option B**: `go.uber.org/zap` - Fast, structured
   - **Option C**: `sirupsen/logrus` - Popular, structured

**Recommended: Use slog + lumberjack for rotation**

```go
// Add to go.mod
require gopkg.in/natefinch/lumberjack.v2 v2.2.1

// Setup in serve.go
import (
    "log/slog"
    "gopkg.in/natefinch/lumberjack.v2"
)

func setupLogging(cfg *config.LoggingConfig) *slog.Logger {
    var handler slog.Handler

    if cfg.File != "" {
        // Log to file with rotation
        writer := &lumberjack.Logger{
            Filename:   cfg.File,
            MaxSize:    cfg.MaxSizeMB,
            MaxBackups: cfg.MaxBackups,
            MaxAge:     cfg.MaxAgeDays,
            Compress:   cfg.Compress,
        }

        handler = slog.NewJSONHandler(writer, &slog.HandlerOptions{
            Level: parseLevel(cfg.Level),
        })
    } else {
        // Log to stderr
        handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
            Level: parseLevel(cfg.Level),
        })
    }

    return slog.New(handler)
}

// Usage
logger := setupLogging(&cfg.Logging)
logger.Info("Loading config", "path", path)
logger.Info("Connected to upstream", "server", serverID, "transport", serverCfg.Transport.Kind)
```

3. Example config:
```yaml
logging:
  level: info           # debug, info, warn, error
  file: /var/log/mcp2/mcp2.log
  maxSizeMB: 100
  maxBackups: 3
  maxAgeDays: 30
  compress: true
```

**Files to Modify**:
- `go.mod` - Add lumberjack dependency
- `internal/config/types.go` - Add LoggingConfig
- `cmd/mcp2/cmd/serve.go` - Setup structured logging
- Replace all `log.Printf` with structured logger throughout codebase
- `example-config.yaml` - Add logging config
- `README.md` - Document logging configuration

**Acceptance Criteria**:
- Logs go to configured file
- Log rotation works (size, age, count)
- Structured JSON logs when file logging enabled
- Human-readable logs when stderr
- Log level filtering works
- All components use structured logger

---

### 6. Custom Server Implementation Metadata

**Problem**: Hub always exposes as "mcp2-hub", can't customize name/description per profile.

**Current State**:
```go
// proxy/hub.go:26-29
server := mcp.NewServer(&mcp.Implementation{
    Name:    "mcp2-hub",
    Version: "0.1.0",
}, nil)
```

**Desired Behavior**:
```yaml
profiles:
  safe:
    description: "LLM-facing safe tools"
    metadata:
      name: "Production Safe Hub"
      description: "Filtered MCP gateway for production LLM access"
    servers:
      # ... server configs
```

**Required Changes**:

1. Add metadata to ProfileConfig:
```go
// config/types.go
type ProfileMetadata struct {
    Name        string `json:"name" yaml:"name"`
    Description string `json:"description" yaml:"description"`
    Version     string `json:"version" yaml:"version"`
}

type ProfileConfig struct {
    Description string                         `json:"description" yaml:"description"`
    Metadata    *ProfileMetadata               `json:"metadata" yaml:"metadata"` // Optional
    Servers     map[string]ServerProfileConfig `json:"servers" yaml:"servers"`
}
```

2. Use metadata in Hub creation:
```go
// proxy/hub.go:25-29
func NewHub(cfg *config.RootConfig, manager *upstream.Manager, profileName string) *Hub {
    // Get profile metadata
    profile := cfg.Profiles[profileName]
    name := "mcp2-hub"
    version := "0.1.0"

    if profile.Metadata != nil {
        if profile.Metadata.Name != "" {
            name = profile.Metadata.Name
        }
        if profile.Metadata.Version != "" {
            version = profile.Metadata.Version
        }
    }

    server := mcp.NewServer(&mcp.Implementation{
        Name:    name,
        Version: version,
    }, nil)

    // ... rest of function
}
```

3. Same for per-server proxies:
```go
// proxy/perserver.go - Allow per-server metadata
// Could add metadata to ServerProfileConfig as well
```

**Files to Modify**:
- `internal/config/types.go` - Add ProfileMetadata
- `internal/proxy/hub.go` - Use profile metadata
- `internal/proxy/perserver.go` - Optionally use server metadata
- `example-config.yaml` - Add metadata examples
- `README.md` - Document metadata configuration

**Acceptance Criteria**:
- Hub exposes custom name when metadata provided
- Defaults to "mcp2-hub" when no metadata
- Version customizable per profile
- MCP clients see custom implementation info

---

## Performance Optimizations (P2)

### 7. Parallel List Aggregation

**Problem**: `hub.go` aggregates list results sequentially. Slow with many upstreams.

**Current State**:
```go
// hub.go:102-127
func (h *Hub) handleToolsList(ctx context.Context) (mcp.Result, error) {
    var allTools []*mcp.Tool

    for _, u := range h.manager.List() {  // SEQUENTIAL
        result, err := u.Session.ListTools(ctx, nil)
        if err != nil {
            continue
        }

        for _, tool := range result.Tools {
            if !h.profileEngine.IsToolAllowed(u.ID, tool.Name) {
                continue
            }
            if h.prefixEnabled {
                tool.Name = fmt.Sprintf("%s:%s", u.ID, tool.Name)
            }
            allTools = append(allTools, tool)
        }
    }

    return &mcp.ListToolsResult{Tools: allTools}, nil
}
```

**Optimized Version**:
```go
// hub.go - Parallel aggregation
func (h *Hub) handleToolsList(ctx context.Context) (mcp.Result, error) {
    upstreams := h.manager.List()

    // Results channel
    type result struct {
        tools []*mcp.Tool
        err   error
    }
    results := make(chan result, len(upstreams))

    // Launch parallel requests
    for _, u := range upstreams {
        go func(upstream *upstream.Upstream) {
            tools, err := h.fetchAndFilterTools(ctx, upstream)
            results <- result{tools: tools, err: err}
        }(u)
    }

    // Collect results
    var allTools []*mcp.Tool
    for i := 0; i < len(upstreams); i++ {
        res := <-results
        if res.err != nil {
            // Log but continue
            continue
        }
        allTools = append(allTools, res.tools...)
    }

    return &mcp.ListToolsResult{Tools: allTools}, nil
}

func (h *Hub) fetchAndFilterTools(ctx context.Context, u *upstream.Upstream) ([]*mcp.Tool, error) {
    result, err := u.Session.ListTools(ctx, nil)
    if err != nil {
        return nil, err
    }

    var tools []*mcp.Tool
    for _, tool := range result.Tools {
        if !h.profileEngine.IsToolAllowed(u.ID, tool.Name) {
            continue
        }
        if h.prefixEnabled {
            tool.Name = fmt.Sprintf("%s:%s", u.ID, tool.Name)
        }
        tools = append(tools, tool)
    }

    return tools, nil
}
```

**Same for Resources and Prompts**

**Files to Modify**:
- `internal/proxy/hub.go` - Parallelize all list handlers

**Acceptance Criteria**:
- List operations complete in parallel
- Timeout applies to slowest upstream, not sum
- Errors in one upstream don't block others
- Results merged correctly
- Thread-safe

---

## Nice to Have (P3)

### 8. Metrics and Health Endpoints

Add admin/observability endpoints:

```go
// cmd/mcp2/cmd/serve.go
mux.HandleFunc("/_mcp2/health", handleHealth)
mux.HandleFunc("/_mcp2/metrics", handleMetrics)
mux.HandleFunc("/_mcp2/config", handleConfigView)  // Show active config (redact secrets)
```

### 9. Request/Response Logging

Add optional logging of all MCP requests/responses for debugging:

```yaml
logging:
  level: info
  file: /var/log/mcp2/mcp2.log
  logRequests: true   # Log all MCP requests
  logResponses: true  # Log all MCP responses
```

### 10. Rate Limiting

Add per-server or per-client rate limiting:

```yaml
servers:
  expensive-api:
    # ...
    rateLimit:
      requests: 100
      per: "1m"  # 100 requests per minute
```

---

## Cleanup Tasks

### Delete Redundant Files

1. **test-client.go** - Temporary integration test file (in .gitignore)
2. **test-per-server.go** - Temporary integration test file (in .gitignore)

**Keep**:
- **test-context7.yaml** - Good example config, keep in repo
- **INTEGRATION_TEST_RESULTS.md** - Valuable documentation

### Update Documentation

1. **SPEC.md Phase 5** - Mark as obsolete, reference mcptools instead
2. **README.md** - Add section on mcptools integration workflow
3. **MCPTOOLS_COMPARISON.md** - Already done ✅

---

## Implementation Priority

**Sprint 1 (Production Critical)**:
1. Upstream authentication (P0.1)
2. SSE transport support (P0.2)
3. Port/host in config (P0.3)

**Sprint 2 (Operations)**:
4. Hot config reload (P1.4)
5. Structured logging (P1.5)
6. Parallel list aggregation (P2.7)

**Sprint 3 (Polish)**:
7. Custom server metadata (P1.6)
8. Metrics/health endpoints (P3.8)
9. Request logging (P3.9)

**Future**:
- Rate limiting (P3.10)
- TUI (nice to have, mcptools has interactive shell)
- Per-request profile selection via headers (advanced)

---

## Success Criteria

mcp2 is production-ready when:

✅ Can authenticate to any upstream MCP server (Bearer, API keys, custom headers)
✅ Supports all common MCP transports (stdio, HTTP, SSE)
✅ Configuration fully in YAML (no CLI-only options)
✅ Config changes don't require restart (hot reload)
✅ Production-grade logging (structured, rotated, levels)
✅ Fast aggregation (parallel list operations)
✅ Observable (health checks, metrics)
✅ Documented integration with mcptools

**When these are done, mcp2 is the networking powerhouse ready for enterprise deployment.**

---

## Resources & References

This section provides comprehensive references for implementing the remaining work. Resources are organized by category with specific file paths, URLs, and key anchor points.

---

### 1. MCP Protocol Specification

**Official Documentation:**
- **MCP Specification (Latest)**: https://modelcontextprotocol.io/specification
- **Protocol Overview**: https://modelcontextprotocol.io/docs/concepts/architecture
- **Transport Specification**: https://modelcontextprotocol.io/specification/2025-03-26/basic/transports
  - Streamable HTTP (recommended): https://spec.modelcontextprotocol.io/specification/2024-11-05/basic/transports/#http-with-sse
  - SSE Transport (legacy 2024-11-05): https://spec.modelcontextprotocol.io/specification/2024-11-05/basic/transports/#http-with-sse
  - Stdio Transport: https://spec.modelcontextprotocol.io/specification/basic/transports/#stdio

**Key Concepts:**
- **Tools**: https://modelcontextprotocol.io/docs/concepts/tools
- **Resources**: https://modelcontextprotocol.io/docs/concepts/resources
- **Prompts**: https://modelcontextprotocol.io/docs/concepts/prompts
- **Sampling**: https://modelcontextprotocol.io/docs/concepts/sampling

**Protocol Messages:**
- JSON-RPC 2.0 format
- Message types: requests, responses, notifications
- Method naming: `tools/list`, `tools/call`, `resources/read`, etc.

---

### 2. Official MCP Go SDK

**Repository:**
- **GitHub**: https://github.com/modelcontextprotocol/go-sdk
- **Go Package**: `github.com/modelcontextprotocol/go-sdk/mcp`
- **Version Used**: v1.1.0 (see go.mod)

**Key SDK Documentation (via `go doc`):**

**Transports:**
```bash
go doc github.com/modelcontextprotocol/go-sdk/mcp.Transport
go doc github.com/modelcontextprotocol/go-sdk/mcp.StreamableClientTransport
go doc github.com/modelcontextprotocol/go-sdk/mcp.SSEClientTransport
go doc github.com/modelcontextprotocol/go-sdk/mcp.CommandTransport  # For stdio
go doc github.com/modelcontextprotocol/go-sdk/mcp.StdioTransport
```

**Core Types:**
```bash
go doc github.com/modelcontextprotocol/go-sdk/mcp.Client
go doc github.com/modelcontextprotocol/go-sdk/mcp.Server
go doc github.com/modelcontextprotocol/go-sdk/mcp.ClientSession
go doc github.com/modelcontextprotocol/go-sdk/mcp.Implementation
```

**Request/Response Types:**
```bash
go doc github.com/modelcontextprotocol/go-sdk/mcp.CallToolParams
go doc github.com/modelcontextprotocol/go-sdk/mcp.CallToolResult
go doc github.com/modelcontextprotocol/go-sdk/mcp.ListToolsParams
go doc github.com/modelcontextprotocol/go-sdk/mcp.ListToolsResult
go doc github.com/modelcontextprotocol/go-sdk/mcp.ReadResourceParams
go doc github.com/modelcontextprotocol/go-sdk/mcp.GetPromptParams
```

**Content Types:**
```bash
go doc github.com/modelcontextprotocol/go-sdk/mcp.Content
go doc github.com/modelcontextprotocol/go-sdk/mcp.TextContent
go doc github.com/modelcontextprotocol/go-sdk/mcp.ImageContent
go doc github.com/modelcontextprotocol/go-sdk/mcp.EmbeddedResource
go doc github.com/modelcontextprotocol/go-sdk/mcp.ResourceContents
```

**Server Creation:**
```bash
go doc github.com/modelcontextprotocol/go-sdk/mcp.NewServer
go doc github.com/modelcontextprotocol/go-sdk/mcp.NewStreamableHTTPHandler
go doc github.com/modelcontextprotocol/go-sdk/mcp.MethodHandler
```

**Critical SDK Fields Referenced in REMAINING_WORK:**
- `StreamableClientTransport.HTTPClient *http.Client` - For auth header injection
- `StreamableClientTransport.MaxRetries int` - For retry logic
- `SSEClientTransport.Endpoint string` - For SSE connections
- `SSEClientTransport.HTTPClient *http.Client` - For SSE auth

---

### 3. f/mcptools (Complementary Tool)

**Repository:**
- **GitHub**: https://github.com/f/mcptools
- **Latest Commit** (as of analysis): 543732d

**Key Blog Post:**
- **"MCP Inspector vs MCP Tools"**: https://blog.fka.dev/blog/2025-03-27-mcp-inspector-vs-mcp-tools/
  - Explains design philosophy
  - Comparison with other MCP tools

**Key Files to Study:**

**Guard Mode Implementation** (Similar to our filtering):
- `pkg/guard/guard_proxy.go` (415 lines)
  - Line 88-112: `IsAllowed()` - Pattern matching logic
  - Line 114-243: Filter response functions for tools/prompts/resources
  - Line 247-387: `Start()` - Main proxy loop, request/response handling
  - Line 311-349: Call-phase blocking for tools/resources/prompts
  - **Key Difference**: Uses JSON unmarshaling, we use SDK types

**Proxy Mode** (Shell script wrapper):
- `pkg/proxy/proxy.go`
  - Shows how to register shell scripts as MCP tools
  - Environment variable passing pattern

**Mock Server**:
- `pkg/mock/mock.go`
  - Example of implementing MCP server from scratch
  - Useful for understanding protocol flow

**Alias Management**:
- `pkg/alias/alias.go`
  - Simple JSON config management (~/.mcpt/aliases.json)
  - Could inspire `mcp2 import-aliases` command

**Transport Detection**:
- Look for SSE vs HTTP detection logic in command handlers
- Shows how to handle multiple transport types

**Limitations to Note:**
- Guard mode is stdio-only (not HTTP)
- Uses different MCP SDK (mark3labs/mcp-go, not official)
- Single-server focus (no aggregation)

---

### 4. Go Libraries for Implementation

**For Hot Config Reload (P1.4):**
- **fsnotify**: https://github.com/fsnotify/fsnotify
  - File watching for config changes
  - Cross-platform (Linux inotify, macOS FSEvents, Windows)
  - Import: `github.com/fsnotify/fsnotify`
  - Example: Watch file, trigger reload on Write event

**For Structured Logging (P1.5):**
- **log/slog** (stdlib, Go 1.21+):
  - Documentation: https://pkg.go.dev/log/slog
  - Structured logging, levels, JSON/text handlers
  - No external dependencies
  
- **lumberjack** (log rotation):
  - GitHub: https://github.com/natefinch/lumberjack
  - Import: `gopkg.in/natefinch/lumberjack.v2`
  - Features: Size/age/count-based rotation, compression
  - Example:
    ```go
    &lumberjack.Logger{
        Filename:   "/var/log/mcp2/mcp2.log",
        MaxSize:    100, // MB
        MaxBackups: 3,
        MaxAge:     30, // days
        Compress:   true,
    }
    ```

**Alternative Logging Libraries:**
- **zap**: https://github.com/uber-go/zap (faster, more features)
- **logrus**: https://github.com/sirupsen/logrus (popular, mature)

**For HTTP Client Customization (P0.1):**
- **net/http.RoundTripper** (stdlib):
  - Documentation: https://pkg.go.dev/net/http#RoundTripper
  - Custom transport for header injection
  - Chain-able middleware pattern

---

### 5. Example MCP Servers (For Testing)

**Context7** (Used in our integration tests):
- **NPM Package**: `@upstash/context7-mcp`
- **Install**: `npx -y @upstash/context7-mcp`
- **Transport**: stdio
- **Tools**: `resolve-library-id`, `get-library-docs`
- **Use Case**: Library documentation retrieval
- **Test Config**: See `test-context7.yaml` in repo

**Official MCP Servers:**
- **Filesystem**: `@modelcontextprotocol/server-filesystem`
  - Tools: read_file, write_file, list_directory, search_files
  - Good for basic stdio testing
  
- **Everything Server** (Docker):
  - `docker run -p 3001:3001 --rm -it tzolov/mcp-everything-server:v1`
  - Provides tools, resources, prompts for comprehensive testing
  - Useful for SSE transport testing (port 3001/sse)

**GitHub MCP Server** (For auth testing):
- **Docker**: `ghcr.io/github/github-mcp-server`
- Requires: `GITHUB_PERSONAL_ACCESS_TOKEN` environment variable
- Good for testing header authentication

---

### 6. Key Files in mcp2 Codebase

**Configuration System:**
- `internal/config/types.go`
  - Line 4-8: `ComponentFilter` - Allow/deny pattern structure
  - Line 10-15: `ServerProfileConfig` - Per-server filters
  - Line 17-30: `ServerTransportConfig` - Transport definitions
  - Line 50-57: `RootConfig` - Main config structure
  - **Add here**: `ServerListenConfig`, `LoggingConfig`, `ProfileMetadata`

- `internal/config/loader.go`
  - Line 13-34: `Load()` - YAML/JSON loading
  - Line 36-58: `ExpandEnvVars()` - Environment variable expansion
  - **Keep in mind**: Env vars already work for auth headers

- `internal/config/validator.go`
  - Line 8-41: `Validate()` - Main validation logic
  - Line 43-78: `validateServerConfig()` - Server-specific validation
  - **Update here**: Add SSE to valid transport kinds

**Upstream Connection Management:**
- `internal/upstream/manager.go`
  - Line 35-83: `Connect()` - Main connection logic
  - Line 51-66: Transport switching (stdio/http)
  - Line 127-140: `createStdioTransport()`
  - Line 143-150: `createHTTPTransport()` **← TODO on line 148**
  - **Critical**: This is where auth headers need to be wired
  - **Add here**: `createSSETransport()`, `headerInjector` type

**Profile-Based Filtering:**
- `internal/profile/engine.go`
  - Line 12-23: `Engine` - Profile engine structure
  - Line 46-80: `isAllowed()` - Core filtering logic (allow/deny)
  - Line 82-91: `matchesAny()` - Pattern matching
  - Line 93-140: `matchPattern()` - Glob pattern support (*, **, filepath)
  - **Used by**: Hub and PerServerProxy for filtering

**Hub Aggregation:**
- `internal/proxy/hub.go`
  - Line 15-22: `Hub` - Main hub structure
  - Line 24-45: `NewHub()` - Hub creation with profile
  - Line 52-99: Middleware registration for tools/resources/prompts
  - Line 101-127: `handleToolsList()` - **Sequential aggregation** (optimize in P2.7)
  - Line 129-179: `handleToolsCall()` - Call routing with filtering
  - Line 181-240: Resource and prompt handlers
  - **Pattern**: All list handlers follow same structure (aggregate + filter)

**Per-Server Proxy:**
- `internal/proxy/perserver.go`
  - Line 13-20: `PerServerProxy` - Isolated server proxy
  - Line 22-37: `NewPerServerProxy()` - Creation with filtering
  - Line 44-86: Handler implementations (tools/resources/prompts)
  - **Key Difference**: No prefixing, direct upstream access

**CLI Commands:**
- `cmd/mcp2/cmd/serve.go`
  - Line 38-157: `runServe()` - Main server logic
  - Line 98-108: HTTP multiplexer setup
  - Line 110-127: Per-server endpoint registration
  - Line 129-148: Graceful shutdown
  - **Update here**: Config reload, logging setup, listen config

- `cmd/mcp2/cmd/profiles.go`
  - Line 25-119: `runProfiles()` - Profile listing
  - Shows how to iterate profiles and display info

- `cmd/mcp2/cmd/call.go`
  - Line 102-110: `connectToMCP2()` - Client connection helper
  - Line 112-187: Tool calling implementation
  - Shows usage of SDK client types

- `cmd/mcp2/cmd/effective.go`
  - Line 28-96: `runEffective()` - Filter inspection
  - Line 98-131: `displayFilterRules()` - Pretty printing
  - Good example of using ProfileEngine programmatically

**Testing:**
- `internal/config/config_test.go` - 12 tests for config loading/validation
- `internal/profile/engine_test.go` - 21 tests for filtering logic
- `internal/proxy/perserver_test.go` - 5 tests for per-server proxy
- **Pattern**: Use testify for assertions, table-driven tests

---

### 7. Reference Implementations & Patterns

**HTTP Header Injection Pattern** (For P0.1):
```go
// Pattern from net/http documentation
type headerInjector struct {
    base    http.RoundTripper
    headers map[string]string
}

func (h *headerInjector) RoundTrip(req *http.Request) (*http.Response, error) {
    // Clone request to avoid mutating original
    req2 := req.Clone(req.Context())
    for k, v := range h.headers {
        req2.Header.Set(k, v)
    }
    return h.base.RoundTrip(req2)
}

// Usage
httpClient := &http.Client{
    Transport: &headerInjector{
        base:    http.DefaultTransport,
        headers: serverCfg.Transport.Headers,
    },
}
```

**File Watching Pattern** (For P1.4):
```go
// Pattern from fsnotify documentation
watcher, _ := fsnotify.NewWatcher()
defer watcher.Close()

watcher.Add(configPath)

for {
    select {
    case event := <-watcher.Events:
        if event.Op&fsnotify.Write == fsnotify.Write {
            // Reload config
        }
    case err := <-watcher.Errors:
        log.Printf("Watcher error: %v", err)
    }
}
```

**Parallel Goroutine Pattern** (For P2.7):
```go
// Pattern for parallel aggregation
type result struct {
    data []*mcp.Tool
    err  error
}

results := make(chan result, len(upstreams))

for _, u := range upstreams {
    go func(upstream *upstream.Upstream) {
        tools, err := fetchTools(ctx, upstream)
        results <- result{data: tools, err: err}
    }(u)
}

var allTools []*mcp.Tool
for i := 0; i < len(upstreams); i++ {
    res := <-results
    if res.err == nil {
        allTools = append(allTools, res.data...)
    }
}
```

---

### 8. Testing Resources

**Integration Test Examples:**
- See `INTEGRATION_TEST_RESULTS.md` for Context7 test results
- Test configuration: `test-context7.yaml`
- Pattern: Start mcp2 server, connect with SDK client, call tools

**Test MCP Endpoints:**
- Public test server: https://ne.tools (mentioned in mcptools docs)
- Docker-based: Use official MCP servers in containers
- Local: npx-based servers for stdio testing

**Testing Auth:**
- GitHub MCP server requires GITHUB_PERSONAL_ACCESS_TOKEN
- Good test case for header authentication
- Can test both header injection and env var expansion

---

### 9. Community Resources

**MCP Discord:**
- Active community for questions
- Protocol clarifications from maintainers

**MCP GitHub Discussions:**
- https://github.com/modelcontextprotocol/specification/discussions
- Design discussions, RFCs

**Example Projects:**
- Search GitHub for "MCP server" or "modelcontextprotocol"
- Many community MCP servers for inspiration

---

### 10. Development Tools

**Useful Commands:**

```bash
# Explore SDK types
go doc -all github.com/modelcontextprotocol/go-sdk/mcp | grep -A5 "type.*Transport"

# Find specific methods
go doc github.com/modelcontextprotocol/go-sdk/mcp | grep -i session

# Check SDK version
go list -m github.com/modelcontextprotocol/go-sdk

# Update SDK
go get -u github.com/modelcontextprotocol/go-sdk

# Build and test
go build ./cmd/mcp2
go test ./...

# Run with race detector
go run -race ./cmd/mcp2 serve -c config.yaml

# Profile performance
go test -cpuprofile cpu.prof -memprofile mem.prof -bench .
```

**Debugging MCP Traffic:**
- Use `--server-logs` flag in mcptools to see server stderr
- Add JSON-RPC logging middleware in mcp2
- Use Wireshark/tcpdump for HTTP transport debugging
- Check `~/.mcpt/logs/` for mcptools logs

---

### 11. Design Decisions & Tradeoffs

**Why Official SDK over mark3labs:**
- Official SDK from modelcontextprotocol org
- Better maintenance and updates
- Closer alignment with spec
- StreamableHTTPHandler built-in

**Why Not Rebuild mcptools Features:**
- mcptools already excellent at exploration/debugging
- Focus on complementary strengths
- Avoid duplication, leverage ecosystem
- See MCPTOOLS_COMPARISON.md for full analysis

**Why YAML over JSON:**
- Comments for documentation
- More human-friendly
- Both supported via tags
- Standard in Kubernetes/Docker ecosystem

**Why Profile-Based vs Flag-Based:**
- Persistent configuration
- Named profiles easier to reference
- Multiple filtering strategies
- Better for enterprise/production
- mcptools guard is for quick one-offs, we're for persistent filtering

---

### 12. Additional Reading

**Go Best Practices:**
- Effective Go: https://go.dev/doc/effective_go
- Go Code Review Comments: https://github.com/golang/go/wiki/CodeReviewComments
- Uber Go Style Guide: https://github.com/uber-go/guide/blob/master/style.md

**Structured Logging:**
- slog tutorial: https://betterstack.com/community/guides/logging/logging-in-go/
- slog handler guide: https://pkg.go.dev/log/slog#Handler

**HTTP Client Patterns:**
- RoundTripper middleware: https://medium.com/@_jesus_rafael/adding-middleware-to-go-http-client-8e5c44d8a0f4
- Context with HTTP: https://blog.golang.org/context

**Configuration Management:**
- Viper library (if needed): https://github.com/spf13/viper
- 12-factor config: https://12factor.net/config

---

## Quick Reference: Most Critical Resources

**For Authentication (P0.1):**
1. `go doc github.com/modelcontextprotocol/go-sdk/mcp.StreamableClientTransport`
2. net/http RoundTripper pattern (see section 7)
3. Our code: `internal/upstream/manager.go:143-150`

**For SSE Support (P0.2):**
1. `go doc github.com/modelcontextprotocol/go-sdk/mcp.SSEClientTransport`
2. MCP SSE spec: https://spec.modelcontextprotocol.io/specification/2024-11-05/basic/transports/#http-with-sse
3. mcptools README examples (SSE transport section)

**For Hot Reload (P1.4):**
1. fsnotify: https://github.com/fsnotify/fsnotify
2. Pattern in section 7
3. Our code: `cmd/mcp2/cmd/serve.go:38-157`

**For Logging (P1.5):**
1. log/slog: https://pkg.go.dev/log/slog
2. lumberjack: https://github.com/natefinch/lumberjack
3. Pattern in REMAINING_WORK P1.5 section

**For Parallel Aggregation (P2.7):**
1. Goroutine pattern in section 7
2. Our code: `internal/proxy/hub.go:101-127` (current sequential version)

---

**Last Updated**: Based on analysis through Phase 4 completion
**Next Review**: After completing P0 critical path items
