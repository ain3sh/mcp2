package upstream

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"mcp2/internal/config"
)

type Upstream struct {
	Config  config.ServerConfig
	Client  *mcp.Client
	Session *mcp.ClientSession
}

type Manager struct {
	upstreams map[string]*Upstream
}

func NewManager(cfg *config.RootConfig) (*Manager, error) {
	mgr := &Manager{
		upstreams: make(map[string]*Upstream),
	}

	for name, srvCfg := range cfg.Servers {
		u, err := NewUpstream(name, srvCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create upstream %q: %w", name, err)
		}
		mgr.upstreams[name] = u
	}

	return mgr, nil
}

func NewUpstream(name string, cfg config.ServerConfig) (*Upstream, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "mcp2-proxy",
		Version: "0.1.0",
	}, nil)

	u := &Upstream{
		Config: cfg,
		Client: client,
	}

	return u, nil
}

type headerTransport struct {
	rt      http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.rt.RoundTrip(req)
}

func (u *Upstream) Connect(ctx context.Context) error {
	var transport mcp.Transport

	switch u.Config.Transport.Kind {
	case "stdio":
		cmd := exec.Command(u.Config.Transport.Command, u.Config.Transport.Args...)
		// Inherit environment variables from the current process
		cmd.Env = os.Environ()
		for k, v := range u.Config.Transport.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
		transport = &mcp.CommandTransport{
			Command: cmd,
		}
	case "http":
		httpClient := &http.Client{}
		if len(u.Config.Transport.Headers) > 0 {
			httpClient.Transport = &headerTransport{
				rt:      http.DefaultTransport,
				headers: u.Config.Transport.Headers,
			}
		}

		transport = &mcp.StreamableClientTransport{
			Endpoint:   u.Config.Transport.URL,
			HTTPClient: httpClient,
		}
	default:
		return fmt.Errorf("unsupported transport kind: %q", u.Config.Transport.Kind)
	}

	session, err := u.Client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	u.Session = session
	return nil
}

func (m *Manager) StartAll(ctx context.Context) error {
	for name, u := range m.upstreams {
		if err := u.Connect(ctx); err != nil {
			return fmt.Errorf("failed to start upstream %q: %w", name, err)
		}
	}
	return nil
}

func (m *Manager) Get(name string) *Upstream {
	return m.upstreams[name]
}

func (m *Manager) GetAll() map[string]*Upstream {
	return m.upstreams
}
