// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"fmt"

	"github.com/nivis-project/nivis/internal/graph"
	"github.com/nivis-project/nivis/internal/ir"
)

// PlanOne plans one resource against its prior state with the given config and
// reports the operation it implies. The set computation needs nothing else from
// the provider, so this is the whole seam.
type PlanOne func(ctx context.Context, node *ir.ResourceNode, cfg map[string]interface{}) (Op, error)

// Withheld is one re-evaluation of the configuration with a set of resources'
// recorded outputs kept back, and the references resolved against what is left.
type Withheld struct {
	Graph   *ir.Graph
	DAG     *graph.DAG
	Configs map[string]map[string]interface{}
}

// Reevaluate re-evaluates the configuration with the given ids' outputs
// withheld. See ReplaceSet for why this cannot be done in process.
type Reevaluate func(ctx context.Context, withheld map[string]bool) (Withheld, error)

// ReplaceSetInput is what computing the set needs.
type ReplaceSetInput struct {
	// Graph and Configs are this phase's evaluation, used for the first round.
	Graph   *ir.Graph
	Configs map[string]map[string]interface{}
	// Candidates are the resources the set may contain. ONLY resources whose
	// configuration is already fully resolved belong here, see ReplaceSet.
	Candidates map[string]bool
	// Known are operations the caller has already computed for this phase's
	// evaluation. The first round is exactly that computation, so a caller that
	// has just done it (a plan report) passes its answers rather than paying
	// for every candidate a second time.
	Known map[string]Op
	// Reeval is required for a cascade to be found at all. nil limits the set
	// to the resources whose own configuration changed.
	Reeval Reevaluate
	Plan   PlanOne
}

// ReplaceSet returns the resources that must be replaced together, in IR order.
//
// A resource that reads an attribute of a replaced resource cannot know that
// attribute's next value. Withholding the replaced resource's outputs leaves
// the reference unresolved, an unresolved reference encodes as unknown, and the
// provider then decides whether that unknown forces a new object. Asking is the
// point: measured against a real provider, an unknown in an identity-bearing
// attribute forces a replacement and an unknown in one the provider can update
// in place does not (bean nixform2-nwnf). Assuming either way is wrong.
//
// Withholding REQUIRES re-evaluating the configuration, and this was the one
// thing the design got wrong before it was measured. Nivis evaluates with the
// recorded outputs injected, and an evaluation that can see a value bakes it in
// as a constant: the reference, and the IR edge recording it, are both gone by
// the time the executor sees the graph. So a dependency cannot be recovered in
// process from an IR that was evaluated with its target known. Dropping the
// target and evaluating again is what brings the reference back.
//
// The first round needs no evaluation, because it asks only which resources
// their own configuration already changes. A run that replaces nothing
// therefore pays nothing, and a cascade costs one evaluation per link.
//
// Only resources already fully resolved are candidates. An unknown that comes
// from withholding an output and an unknown that comes from a reference this
// phase has not resolved are indistinguishable once encoded, so a resource
// waiting on a datasource would otherwise look like a dependent of something
// being replaced. Restricting the candidates means every unknown the fixpoint
// sees is one it created. It also keeps the set to resources the apply can
// recreate in the same phase.
//
// It terminates: the set only grows and is bounded by the candidate count.
func ReplaceSet(ctx context.Context, in ReplaceSetInput) ([]string, error) {
	if in.Plan == nil {
		return nil, fmt.Errorf("replace set: no planner")
	}
	member := map[string]bool{}

	// Round one, against this phase's evaluation: who does the configuration
	// itself replace?
	for _, id := range in.Graph.Order {
		if !in.Candidates[id] {
			continue
		}
		op, cached := Op(0), false
		if in.Known != nil {
			op, cached = in.Known[id]
		}
		if !cached {
			var err error
			op, err = in.Plan(ctx, in.Graph.Nodes[id], in.Configs[id])
			if err != nil {
				return nil, fmt.Errorf("plan %q while deciding whether it must be replaced: %w", id, err)
			}
		}
		if op == OpReplace {
			member[id] = true
		}
	}
	if len(member) == 0 || in.Reeval == nil {
		return sortedMembers(in.Graph, member), nil
	}

	// Later rounds, each against a fresh evaluation with the set withheld.
	for {
		w, err := in.Reeval(ctx, member)
		if err != nil {
			return nil, fmt.Errorf("re-evaluate with the replaced resources withheld: %w", err)
		}
		grew := false
		for _, id := range w.Graph.Order {
			if member[id] || !in.Candidates[id] {
				continue
			}
			if !dependsOnAny(w.DAG, id, member) {
				continue
			}
			op, err := in.Plan(ctx, w.Graph.Nodes[id], w.Configs[id])
			if err != nil {
				return nil, fmt.Errorf("plan %q while computing what must be replaced with it: %w", id, err)
			}
			if op == OpReplace {
				member[id] = true
				grew = true
			}
		}
		if !grew {
			return sortedMembers(w.Graph, member), nil
		}
	}
}

func sortedMembers(g *ir.Graph, member map[string]bool) []string {
	var out []string
	seen := map[string]bool{}
	for _, id := range g.Order {
		if member[id] {
			out = append(out, id)
			seen[id] = true
		}
	}
	// A member the latest evaluation no longer declares is still in the set: it
	// is in state and its prior object must go.
	for id := range member {
		if !seen[id] {
			out = append(out, id)
		}
	}
	return out
}

// dependsOnAny reports whether id reads anything in the given set, directly or
// transitively. Reversing Dependents rather than walking Deps keeps one notion
// of reachability in the DAG.
func dependsOnAny(d *graph.DAG, id string, set map[string]bool) bool {
	if d == nil || len(set) == 0 {
		return false
	}
	for member := range set {
		for _, dep := range d.Dependents(member) {
			if dep == id {
				return true
			}
		}
	}
	return false
}
