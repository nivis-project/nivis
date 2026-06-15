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

	"github.com/wearetechnative/nixform/internal/tfplugin6"
)

// handshake matches the values tfprotov6's tf6server serves with. They MUST be
// identical or go-plugin refuses the connection.
var handshake = goplugin.HandshakeConfig{
	ProtocolVersion:  6,
	MagicCookieKey:   "TF_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "d602bf8f470bc67ca7faa0386276bbdd4330efaf76d1a219cb4d6991ca9872b2",
}

// grpcProviderPlugin is the client half of the go-plugin GRPCPlugin interface:
// its GRPCClient returns a tfplugin6.ProviderClient over the dialed connection.
type grpcProviderPlugin struct {
	goplugin.NetRPCUnsupportedPlugin
}

func (grpcProviderPlugin) GRPCServer(*goplugin.GRPCBroker, *grpc.Server) error {
	return fmt.Errorf("nixform is a plugin client, not a server")
}

func (grpcProviderPlugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return tfplugin6.NewProviderClient(c), nil
}

// pluginSet is the v6 plugin map; the key "provider" matches tf6server.
var pluginSet = goplugin.PluginSet{"provider": grpcProviderPlugin{}}

// Manager spawns and pools provider processes, keyed by provider identity.
type Manager struct {
	mu      sync.Mutex
	clients map[string]*entry
}

type entry struct {
	client   *goplugin.Client
	provider tfplugin6.ProviderClient
}

// NewManager returns an empty manager.
func NewManager() *Manager { return &Manager{clients: map[string]*entry{}} }

// Client spawns (or reuses) the provider binary at path under the given identity
// and returns a connected tfprotov6 provider client. Reusing by identity means
// two resources of the same provider share one process.
func (m *Manager) Client(identity, path string) (tfplugin6.ProviderClient, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if e, ok := m.clients[identity]; ok {
		return e.provider, nil
	}

	c := goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  handshake,
		VersionedPlugins: map[int]goplugin.PluginSet{6: pluginSet},
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
	prov, ok := raw.(tfplugin6.ProviderClient)
	if !ok {
		c.Kill()
		return nil, fmt.Errorf("plugin %q: unexpected client type %T", identity, raw)
	}

	m.clients[identity] = &entry{client: c, provider: prov}
	return prov, nil
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
