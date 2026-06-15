// Package refresh reconciles stored state with the provider's view via the
// provider's read operation. It does not plan or apply. Version-neutral.
package refresh

import (
	"context"
	"fmt"

	"github.com/wearetechnative/nixform/internal/ir"
	"github.com/wearetechnative/nixform/internal/plan"
	"github.com/wearetechnative/nixform/internal/provider"
	"github.com/wearetechnative/nixform/internal/state"
)

// Manager is the provider-client seam (internal/plugin.Manager satisfies it).
type Manager interface {
	Client(identity, path string) (provider.Client, error)
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
			continue // state for a resource no longer in the config; leave as-is.
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
	out, err := client.Read(ctx, provider.ReadRequest{
		Schema:   rs,
		TypeName: node.Resource.Type,
		Stored:   stored.Attrs,
	})
	if err != nil {
		return nil, err
	}
	return out.Attrs, nil
}
