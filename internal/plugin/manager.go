// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package plugin spawns provider binaries and speaks the Terraform plugin
// protocol (tfprotov6 over go-plugin/gRPC) to them as a client. We spawn
// unmodified provider binaries and talk the protocol; we do not link provider
// source (DESIGN D2).
package plugin

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/hashicorp/go-hclog"
	goplugin "github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/nivis-project/nivis/internal/provider"
	v5 "github.com/nivis-project/nivis/internal/provider/v5"
	v6 "github.com/nivis-project/nivis/internal/provider/v6"
	"github.com/nivis-project/nivis/internal/providerlog"
	"github.com/nivis-project/nivis/internal/tfplugin5"
	"github.com/nivis-project/nivis/internal/tfplugin6"
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
type v6Plugin struct {
	goplugin.NetRPCUnsupportedPlugin
}

func (v6Plugin) GRPCServer(*goplugin.GRPCBroker, *grpc.Server) error {
	return fmt.Errorf("nivis is a plugin client, not a server")
}
func (v6Plugin) GRPCClient(_ context.Context, _ *goplugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return tfplugin6.NewProviderClient(c), nil
}

type v5Plugin struct {
	goplugin.NetRPCUnsupportedPlugin
}

func (v5Plugin) GRPCServer(*goplugin.GRPCBroker, *grpc.Server) error {
	return fmt.Errorf("nivis is a plugin client, not a server")
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
	mu       sync.Mutex
	clients  map[string]*entry
	resolver Resolver
	// logSink renders spawned providers' log entries as readable notes. nil
	// discards them (a caller that wants no provider output at all, e.g. a test).
	logSink *providerlog.Sink
	// logLevel selects which entry levels reach the sink; entries below it are
	// dropped by hclog before they are formatted, and go-plugin skips preparing
	// them at all, which is what keeps a trace-happy provider cheap.
	logLevel providerlog.Level
}

type entry struct {
	client   *goplugin.Client
	provider provider.Client
}

// Resolver turns a provider source into a local binary path. A registry-backed
// resolver fetches+verifies+caches by address; a filesystem path is returned
// as-is. nil means "use the source verbatim".
type Resolver interface {
	ResolveProvider(ctx context.Context, source string) (string, error)
}

// NewManager returns an empty manager (no resolver: sources used verbatim).
func NewManager() *Manager {
	return &Manager{clients: map[string]*entry{}, logLevel: providerlog.DefaultLevel}
}

// WithProviderLog sets the sink that renders spawned providers' log entries and
// the level at which they are surfaced, returning the manager for chaining. A
// nil sink discards provider logs entirely.
//
// This is the ONLY route by which a provider's log output can reach a user:
// go-plugin's ClientConfig.Stderr defaults to io.Discard, so the logger installed
// here is the single interception point (see internal/providerlog).
func (m *Manager) WithProviderLog(sink *providerlog.Sink, level providerlog.Level) *Manager {
	m.logSink = sink
	m.logLevel = level
	return m
}

// WithResolver sets the provider-source resolver (e.g. the registry client) and
// returns the manager for chaining.
func (m *Manager) WithResolver(r Resolver) *Manager { m.resolver = r; return m }

// Client spawns (or reuses) the provider binary at path under the given identity
// and returns a version-neutral provider.Client, **configured** with the given
// provider config. Use this for plan/apply/refresh/destroy. Reusing by identity
// means two resources of the same provider share one process; configuration
// happens once, on first spawn. go-plugin negotiates the protocol (v5 or v6) and
// the matching backend is built.
func (m *Manager) Client(identity, path string, config map[string]interface{}) (provider.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cl, c, reused, err := m.dispense(identity, path)
	if err != nil {
		return nil, err
	}
	if reused {
		return cl, nil
	}

	// Configure the provider once, before it is used for plan/apply. An empty
	// config is a valid no-op for config-free providers (the fakes).
	if err := cl.Configure(context.Background(), config); err != nil {
		c.Kill()
		return nil, fmt.Errorf("plugin %q: configure: %w", identity, err)
	}

	m.clients[identity] = &entry{client: c, provider: cl}
	return cl, nil
}

// ClientForSchema returns a provider client WITHOUT configuring it, for fetching
// the schema (codegen). GetProviderSchema does not require configuration per the
// plugin protocol, so this works even against providers that reject an
// unconfigured configure (credential-requiring providers such as
// proxmox/azurerm/google). plan/apply must use Client (which configures); this
// path is schema-only and never calls Configure.
func (m *Manager) ClientForSchema(identity, path string) (provider.Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cl, c, reused, err := m.dispense(identity, path)
	if err != nil {
		return nil, err
	}
	if reused {
		return cl, nil
	}
	m.clients[identity] = &entry{client: c, provider: cl}
	return cl, nil
}

// dispense spawns (or reuses) the provider, performs the handshake, dispenses the
// gRPC client, and builds the version-neutral backend, WITHOUT configuring. It is
// the shared body of Client and ClientForSchema; the caller (holding m.mu) decides
// whether to configure. When an existing pooled client is reused, reused is true
// and c is nil. Callers must hold m.mu.
func (m *Manager) dispense(identity, path string) (cl provider.Client, c *goplugin.Client, reused bool, err error) {
	if e, ok := m.clients[identity]; ok {
		return e.provider, nil, true, nil
	}

	// Resolve the source to a local binary path (registry address -> fetched +
	// verified + cached binary; a filesystem path passes through).
	if m.resolver != nil {
		resolved, rerr := m.resolver.ResolveProvider(context.Background(), path)
		if rerr != nil {
			return nil, nil, false, fmt.Errorf("plugin %q: resolve %q: %w", identity, path, rerr)
		}
		path = resolved
	}

	c = goplugin.NewClient(&goplugin.ClientConfig{
		HandshakeConfig:  handshake,
		VersionedPlugins: versionedPlugins,
		Cmd:              exec.Command(path),
		AllowedProtocols: []goplugin.Protocol{goplugin.ProtocolGRPC},
		Managed:          false,
		// Provider log output goes through the renderer, never to a terminal raw.
		// hclog is used as a SERIALIZER here (JSONFormat) whose Output is the
		// sink: level filtering stays at the source, so a provider emitting
		// enormous TRACE/DEBUG during schema fetch costs nothing, while what does
		// pass the filter is rendered as a readable note rather than printed as
		// the provider's internal telemetry.
		//
		// Stderr is left at its io.Discard default deliberately: go-plugin writes
		// every raw line there before any levelling, so pointing it anywhere else
		// would reintroduce exactly the unrendered output this replaces.
		Logger: hclog.New(&hclog.LoggerOptions{
			Name:       "provider",
			Level:      m.logLevel.HCLog(),
			JSONFormat: true,
			Output:     m.logWriter(identity),
		}),
	})

	rpcClient, err := c.Client()
	if err != nil {
		c.Kill()
		return nil, nil, false, fmt.Errorf("plugin %q: handshake: %w", identity, err)
	}
	raw, err := rpcClient.Dispense("provider")
	if err != nil {
		c.Kill()
		return nil, nil, false, fmt.Errorf("plugin %q: dispense: %w", identity, err)
	}

	// Build the backend matching the negotiated protocol version.
	switch c.NegotiatedVersion() {
	case 5:
		rawClient, ok := raw.(tfplugin5.ProviderClient)
		if !ok {
			c.Kill()
			return nil, nil, false, fmt.Errorf("plugin %q: negotiated v5 but got %T", identity, raw)
		}
		cl = v5.New(rawClient)
	case 6:
		rawClient, ok := raw.(tfplugin6.ProviderClient)
		if !ok {
			c.Kill()
			return nil, nil, false, fmt.Errorf("plugin %q: negotiated v6 but got %T", identity, raw)
		}
		cl = v6.New(rawClient)
	default:
		c.Kill()
		return nil, nil, false, fmt.Errorf("plugin %q: unsupported negotiated protocol version %d", identity, c.NegotiatedVersion())
	}
	return cl, c, false, nil
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

// logWriter is the hclog Output for one spawned provider: the sink's writer for
// this identity, or a discard when no sink is configured. The identity comes from
// the manager rather than the entry, because an entry's @module is the provider's
// own inner module (e.g. sdk.helper_schema), not the provider itself.
func (m *Manager) logWriter(identity string) io.Writer {
	if m.logSink == nil {
		return io.Discard
	}
	return m.logSink.Writer(identity)
}
