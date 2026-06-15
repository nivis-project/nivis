// Copyright 2026 WeareTechnative B.V. and the nixform authors
// SPDX-License-Identifier: Apache-2.0

// Package destroy tears down applied resources in reverse dependency order via
// the provider's destroy operation, then removes them from the state store.
// Version-neutral (provider.Client).
package destroy

import (
	"context"
	"fmt"

	"github.com/wearetechnative/nixform/internal/graph"
	"github.com/wearetechnative/nixform/internal/ir"
	"github.com/wearetechnative/nixform/internal/plan"
	"github.com/wearetechnative/nixform/internal/provider"
	"github.com/wearetechnative/nixform/internal/state"
)

// Manager is the provider-client seam (internal/plugin.Manager satisfies it).
type Manager interface {
	Client(identity, path string) (provider.Client, error)
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
	_, err = client.Destroy(ctx, provider.DestroyRequest{
		Schema:   rs,
		TypeName: node.Resource.Type,
		Stored:   stored.Attrs,
	})
	return err
}
