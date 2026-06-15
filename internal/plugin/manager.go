// Package plugin spawns provider binaries and speaks the Terraform plugin
// protocol (tfprotov6 over go-plugin/gRPC) to them as a client. We spawn
// unmodified provider binaries and talk the protocol; we do not link provider
// source (DESIGN D2).
package plugin

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/wearetechnative/nixform/internal/provider"
	v5 "github.com/wearetechnative/nixform/internal/provider/v5"
	v6 "github.com/wearetechnative/nixform/internal/provider/v6"
	"github.com/wearetechnative/nixform/internal/tfplugin5"
	"github.com/wearetechnative/nixform/internal/tfplugin6"
)

// handshake matches the values tfprotov6's tf6server serves with. They MUST be
// identical or go-plugin refuses the connection.
var handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  6,
	MagicCookieKey:   "TF_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "d602bf8f470bc67ca7faa0386276bbdd4330efaf76d1a219cb4d6991ca9872b2",
}

// v6Plugin / v5Plugin are the client halves of the go-plugin GRPCPlugin
// interface for each protocol version: GRPCClient returns the matching generated
// provider client over the dialed connection.
type v6Plugin struct{ goplugin.NetRPCUnsupportedPlugin }

func (v6Plugin) GRPCServer(*goplugin.GRPCBroker, *grpc.Server) error {
	return fmt.Errorf("nixform is a plugin client, not a server")
}
func (v6Plugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return tfplugin6.NewProviderClient(c), nil
}

type v5Plugin struct{ goplugin.NetRPCUnsupportedPlugin }

func (v5Plugin) GRPCServer(*goplugin.GRPCBroker, *grpc.Server) error {
	return fmt.Errorf("nixform is a plugin client, not a server")
}
func (v5Plugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return tfplugin5.NewProviderClient(c), nil
}

// versionedPlugins offers both protocols; go-plugin negotiates the one the
// provider serves. The key "provider" matches both tf5server and tf6server.
var versionedPlugins = map[int]goplugin.PluginSet{
	5: {"provider": v5Plugin{}},
	6: {"provider": v6Plugin{}},
}

// Manager spawns and pools provider processes, keyed by provider identity.
type Manager struct {
	mu      sync.Mutex
	clients map[string]*entry
}

type entry struct {
	client   *goplugin.Client
	provider provider.Client
}

// NewManager returns an empty manager.
func NewManager() *Manager { return &Manager{clients: map[string]*entry{}} }

// Client spawns (or reuses) the provider binary at path under the given identity
// and returns a version-neutral provider.Client. Reusing by identity means two
// resources of the same provider share one process. For now only the tfprotov6
// backend is wired; a tfprotov5 backend plugs in here behind the same interface.
func (m *Manager) Client(identity, path string) (provider.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if e, ok := m.clients[identity]; ok {
		return e.provider, nil
	}

	c := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  handshake,
		VersionedPlugins: versionedPlugins,
		Cmd:              exec.Command(path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Managed:          false,
	})

	rpcClient, err := c.Client()
	if err != nil {
		c.Kill()
		return nil, fmt.Errorf("plugin %q: handshake: %w", identity, err)
	}
	raw, err := rpcClient.Dispense("provider")
	if err != nil {
		c.Kill()
		return nil, fmt.Errorf("plugin %q: dispense: %w", identity, err)
	}

	// Build the backend matching the negotiated protocol version.
	var cl provider.Client
	switch c.NegotiatedVersion() {
	case 5:
		rawClient, ok := raw.(tfplugin5.ProviderClient)
		if !ok {
			c.Kill()
			return nil, fmt.Errorf("plugin %q: negotiated v5 but got %T", identity, raw)
		}
		cl = v5.New(rawClient)
	case 6:
		rawClient, ok := raw.(tfplugin6.ProviderClient)
		if !ok {
			c.Kill()
			return nil, fmt.Errorf("plugin %q: negotiated v6 but got %T", identity, raw)
		}
		cl = v6.New(rawClient)
	default:
		c.Kill()
		return nil, fmt.Errorf("plugin %q: unsupported negotiated protocol version %d", identity, c.NegotiatedVersion())
	}

	m.clients[identity] = &entry{client: c, provider: cl}
	return cl, nil
}

// Close kills all spawned provider processes.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, e := range m.clients {
		e.client.Kill()
	}
	m.clients = map[string]*entry{}
}
