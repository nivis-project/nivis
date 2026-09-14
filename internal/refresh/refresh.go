// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package refresh reconciles stored state with the provider's view via the
// provider's read operation. It does not plan or apply. Version-neutral.
package refresh

import (
	"context"
	"fmt"
	"time"

	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/progress"
	"github.com/nivis-project/nivis/internal/provider"
	"github.com/nivis-project/nivis/internal/state"
)

// Manager is the provider-client seam (internal/plugin.Manager satisfies it).
type Manager interface {
	Client(identity, path string, config map[string]interface{}) (provider.Client, error)
}

// Result reports which resources were reconciled.
type Result struct {
	Refreshed []string
	// Drifted lists the resources whose real state differed from what was
	// stored. It is the part of a refresh worth reporting: the count refreshed
	// says what happened, this says what CHANGED.
	Drifted []string
}

// Options tune a refresh run.
type Options struct {
	// Observer, if set, receives a progress event per resource as it is read.
	// nil discards.
	Observer progress.Observer
}

// Run refreshes every resource that exists in both the graph and the state store.
func Run(ctx context.Context, g *ir.Graph, mgr Manager, store state.Store, opts Options) (*Result, error) {
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
		progress.Emit(opts.Observer, progress.Event{
			Kind: progress.NodeStart, ID: rs.ID, Type: rs.Type, Refresh: true,
		})
		started := time.Now()
		newAttrs, err := refreshOne(ctx, g, mgr, node, rs)
		if err != nil {
			err = fmt.Errorf("refresh %q: %w", rs.ID, err)
			progress.Emit(opts.Observer, progress.Event{
				Kind: progress.NodeDone, ID: rs.ID, Type: rs.Type, Refresh: true,
				Duration: time.Since(started), Err: err,
			})
			return res, err
		}
		drifted := state.Drifted(rs.Attrs, newAttrs)
		if err := store.Set(state.ResourceState{ID: rs.ID, Type: rs.Type, Attrs: newAttrs}); err != nil {
			err = fmt.Errorf("refresh %q: write state: %w", rs.ID, err)
			progress.Emit(opts.Observer, progress.Event{
				Kind: progress.NodeDone, ID: rs.ID, Type: rs.Type, Refresh: true,
				Duration: time.Since(started), Err: err,
			})
			return res, err
		}
		progress.Emit(opts.Observer, progress.Event{
			Kind: progress.NodeDone, ID: rs.ID, Type: rs.Type, Refresh: true,
			Drifted: drifted, Duration: time.Since(started),
		})
		res.Refreshed = append(res.Refreshed, rs.ID)
		if drifted {
			res.Drifted = append(res.Drifted, rs.ID)
		}
	}
	return res, nil
}

func refreshOne(ctx context.Context, g *ir.Graph, mgr Manager, node *ir.ResourceNode, stored state.ResourceState) (map[string]interface{}, error) {
	prov := g.Providers[node.Resource.Provider]
	client, err := mgr.Client(node.Resource.Provider, prov.Source, prov.Config)
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
