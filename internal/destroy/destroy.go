// Package destroy tears down applied resources in reverse dependency order by
// calling ApplyResourceChange with a null planned state (DESIGN: the provider
// deletes the resource), then removing them from the state store.
package destroy

import (
	"context"
	"fmt"
	"strings"

	"github.com/wearetechnative/nixform/internal/graph"
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

// Options tune a destroy run.
type Options struct {
	// Target, if non-empty, restricts the destroy to a single resource id.
	Target string
}

// Result reports what was destroyed.
type Result struct {
	Destroyed []string // resource ids, in the order they were destroyed
}

// Run destroys resources in reverse dependency order. Only resources present in
// the state store are destroyed. preventDestroy resources cause a named error
// and are left in state.
func Run(ctx context.Context, g *ir.Graph, mgr Manager, store state.Store, opts Options) (*Result, error) {
	dag, err := graph.Build(g)
	if err != nil {
		return nil, err
	}

	res := &Result{}
	for _, id := range dag.DestroyOrder() {
		if opts.Target != "" && id != opts.Target {
			continue
		}
		node, ok := g.Nodes[id]
		if !ok {
			continue
		}
		// Only destroy what we actually have state for.
		stored, found, err := store.Get(id)
		if err != nil {
			return res, err
		}
		if !found {
			continue
		}
		if node.Resource.Meta != nil && node.Resource.Meta.Lifecycle != nil && node.Resource.Meta.Lifecycle.PreventDestroy {
			return res, fmt.Errorf("destroy: %q has lifecycle.preventDestroy set; refusing to destroy", id)
		}

		if err := destroyOne(ctx, g, mgr, node, stored); err != nil {
			return res, fmt.Errorf("destroy %q: %w", id, err)
		}
		if err := store.Delete(id); err != nil {
			return res, fmt.Errorf("destroy %q: remove state: %w", id, err)
		}
		res.Destroyed = append(res.Destroyed, id)
	}
	return res, nil
}

func destroyOne(ctx context.Context, g *ir.Graph, mgr Manager, node *ir.ResourceNode, stored state.ResourceState) error {
	prov, ok := g.Providers[node.Resource.Provider]
	if !ok {
		return fmt.Errorf("provider %q not declared", node.Resource.Provider)
	}
	client, err := mgr.Client(node.Resource.Provider, prov.Source)
	if err != nil {
		return err
	}
	rs, err := plan.SchemaFor(ctx, client, node.Resource.Type)
	if err != nil {
		return err
	}
	prior, err := tfvalue.EncodeState(rs.ObjType, stored.Attrs)
	if err != nil {
		return fmt.Errorf("encode prior state: %w", err)
	}
	nullPlanned, err := tfvalue.NullState(rs.ObjType)
	if err != nil {
		return err
	}
	resp, err := client.ApplyResourceChange(ctx, &tfplugin6.ApplyResourceChange_Request{
		TypeName:     node.Resource.Type,
		PriorState:   prior,
		PlannedState: nullPlanned,
		Config:       nullPlanned,
	})
	if err != nil {
		return fmt.Errorf("ApplyResourceChange(delete): %w", err)
	}
	return diagErr(resp.GetDiagnostics())
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
