package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ain3sh/mcp2/internal/config"
	"github.com/ain3sh/mcp2/internal/proxy"
	"github.com/ain3sh/mcp2/internal/upstream"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

var (
	port  int
	stdio bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the MCP proxy server",
	Long:  `Start the mcp2 proxy server, connecting to upstream servers and exposing a filtered MCP interface.`,
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntVarP(&port, "port", "", 8210, "port to listen on")
	serveCmd.Flags().BoolVarP(&stdio, "stdio", "", false, "use stdio transport instead of HTTP")
}

func runServe(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Expand config path
	path := expandPath(configPath)

	log.Printf("Loading config from: %s", path)

	// Load and validate config
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cfg.ExpandEnvVars()

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Determine active profile
	activeProfile := cfg.DefaultProfile
	if profileName != "" {
		activeProfile = profileName
	}

	if _, ok := cfg.Profiles[activeProfile]; !ok {
		return fmt.Errorf("profile %q not found", activeProfile)
	}

	log.Printf("Using profile: %s", activeProfile)

	// Create upstream manager
	manager := upstream.NewManager()

	// Connect to all servers
	for serverID, serverCfg := range cfg.Servers {
		log.Printf("Connecting to upstream server: %s (%s)", serverID, serverCfg.DisplayName)
		if err := manager.Connect(ctx, serverID, &serverCfg); err != nil {
			return fmt.Errorf("failed to connect to server %q: %w", serverID, err)
		}
		log.Printf("  Connected to %s via %s transport", serverID, serverCfg.Transport.Kind)
	}

	defer manager.Close()

	// Create hub server if enabled
	if !cfg.Hub.Enabled {
		return fmt.Errorf("hub must be enabled in config")
	}

	hub := proxy.NewHub(cfg, manager)

	if stdio {
		// Run in stdio mode
		log.Println("Starting mcp2 hub in stdio mode")
		return hub.Server().Run(ctx, &mcp.StdioTransport{})
	}

	// Run in HTTP mode
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	log.Printf("Starting mcp2 hub on http://%s/mcp", addr)

	// Create HTTP handler
	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return hub.Server()
	}, nil)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// Handle graceful shutdown
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGTERM)
		<-sigint

		log.Println("Shutting down server...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}()

	// Start server
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	log.Println("Server stopped")
	return nil
}
