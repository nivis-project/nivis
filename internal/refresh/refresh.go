// Package refresh reconciles stored state with the provider's view by calling
// ReadResource for each resource in state and writing back the result. It does
// not plan or apply changes.
package refresh

import (
	"context"
	"fmt"
	"strings"

	"github.com/wearetechnative/nixform/internal/ir"
	"github.com/wearetechnative/nixform/internal/plan"
	"github.com/wearetechnative/nixform/internal/state"
	"github.com/wearetechnative/nixform/internal/tfplugin6"
	"github.com/wearetechnative/nixform/internal/tfvalue"
)

// Manager is the provider-client seam (internal/plugin.Manager satisfies it).
type Manager interface {
	Client(identity, path string) (tfplugin6.ProviderClient, error)
}

// Result reports which resources were reconciled.
type Result struct {
	Refreshed []string
}

// Run refreshes every resource that exists in both the graph and the state store.
func Run(ctx context.Context, g *ir.Graph, mgr Manager, store state.Store) (*Result, error) {
	stored, err := store.List()
	if err != nil {
		return nil, err
	}
	res := &Result{}
	for _, rs := range stored {
		node, ok := g.Nodes[rs.ID]
		if !ok {
			// state for a resource no longer in the config; leave it as-is.
			continue
		}
		newAttrs, err := refreshOne(ctx, g, mgr, node, rs)
		if err != nil {
			return res, fmt.Errorf("refresh %q: %w", rs.ID, err)
		}
		if err := store.Set(state.ResourceState{ID: rs.ID, Type: rs.Type, Attrs: newAttrs}); err != nil {
			return res, fmt.Errorf("refresh %q: write state: %w", rs.ID, err)
		}
		res.Refreshed = append(res.Refreshed, rs.ID)
	}
	return res, nil
}

func refreshOne(ctx context.Context, g *ir.Graph, mgr Manager, node *ir.ResourceNode, stored state.ResourceState) (map[string]interface{}, error) {
	prov := g.Providers[node.Resource.Provider]
	client, err := mgr.Client(node.Resource.Provider, prov.Source)
	if err != nil {
		return nil, err
	}
	rs, err := plan.SchemaFor(ctx, client, node.Resource.Type)
	if err != nil {
		return nil, err
	}
	current, err := tfvalue.EncodeState(rs.ObjType, stored.Attrs)
	if err != nil {
		return nil, fmt.Errorf("encode current state: %w", err)
	}
	resp, err := client.ReadResource(ctx, &tfplugin6.ReadResource_Request{
		TypeName:     node.Resource.Type,
		CurrentState: current,
	})
	if err != nil {
		return nil, fmt.Errorf("ReadResource: %w", err)
	}
	if err := diagErr(resp.GetDiagnostics()); err != nil {
		return nil, err
	}
	return tfvalue.DecodeState(rs.ObjType, resp.GetNewState())
}

func diagErr(diags []*tfplugin6.Diagnostic) error {
	var msgs []string
	for _, d := range diags {
		if d.GetSeverity() == tfplugin6.Diagnostic_ERROR {
			msgs = append(msgs, strings.TrimSpace(d.GetSummary()+": "+d.GetDetail()))
		}
	}
	if len(msgs) > 0 {
		return fmt.Errorf("provider diagnostics: %s", strings.Join(msgs, "; "))
	}
	return nil
}
