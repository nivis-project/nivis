// Copyright 2026 WeareTechnative B.V. and the nixform authors
// SPDX-License-Identifier: Apache-2.0

// Package graph builds the executor's dependency DAG from an ingested IR and
// resolves the references the executor can resolve itself (TF->TF). It is pure:
// no provider contact, no I/O.
package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wearetechnative/nixform/internal/ir"
)

// DAG is a dependency graph over resource ids. An edge from A to B means "B
// depends on A" (A must complete before B is ready).
type DAG struct {
	nodes []string            // resource ids, deterministic (IR order)
	deps  map[string][]string // id -> sorted list of ids it depends on
}

// Build constructs the DAG from TF->TF refs, explicit IR edges, and dependsOn.
// *->Nix (derived) leaves do NOT create executor dependencies: their resolution
// happens via re-eval, outside the executor's apply ordering. Returns a cycle
// error (naming the involved ids) if the dependency graph is cyclic.
func Build(g *ir.Graph) (*DAG, error) {
	depSet := make(map[string]map[string]bool, len(g.Order))
	for _, id := range g.Order {
		depSet[id] = map[string]bool{}
	}

	addDep := func(dependent, dependency string) {
		if dependent == dependency {
			return
		}
		if depSet[dependent] == nil {
			depSet[dependent] = map[string]bool{}
		}
		depSet[dependent][dependency] = true
	}

	// References create dependencies. A TF->TF __ref names its target directly.
	// A __derived value depends on the resources its inputs come from — this is
	// a real ordering dependency (create after, destroy before) even though the
	// value is resolved via Nix re-eval and disappears from the IR once concrete.
	// We therefore derive ordering from the (unresolved) refs recorded at ingest,
	// not from whether a leaf is still a placeholder in this phase's IR.
	for _, n := range g.Nodes {
		for _, r := range n.Refs {
			switch r.Class {
			case ir.ClassTFTF:
				if r.Target != "" {
					addDep(n.Resource.ID, r.Target)
				}
			case ir.ClassStarToNix:
				for _, in := range r.Inputs {
					if dep := resourceIDOf(in, g); dep != "" {
						addDep(n.Resource.ID, dep)
					}
				}
			}
		}
	}
	// Explicit IR edges: `to` depends on `from`.
	for _, e := range g.Edges {
		addDep(e.To, e.From)
	}
	// depends_on.
	for _, id := range g.Order {
		n := g.Nodes[id]
		if n.Resource.Meta != nil {
			for _, dep := range n.Resource.Meta.DependsOn {
				addDep(id, dep)
			}
		}
	}

	d := &DAG{nodes: append([]string{}, g.Order...)}
	d.deps = make(map[string][]string, len(depSet))
	for id, set := range depSet {
		var deps []string
		for dep := range set {
			deps = append(deps, dep)
		}
		sort.Strings(deps)
		d.deps[id] = deps
	}

	if cyc := d.findCycle(); cyc != nil {
		return nil, fmt.Errorf("graph: dependency cycle: %v", cyc)
	}
	return d, nil
}

// resourceIDOf maps a derived input key "<id>.<attr>" to the resource id it
// refers to. Resource ids contain dots (provider.type.name), so we match the
// longest prefix of the key that is an actual resource id in the graph.
func resourceIDOf(input string, g *ir.Graph) string {
	best := ""
	for id := range g.Nodes {
		if input == id || strings.HasPrefix(input, id+".") {
			if len(id) > len(best) {
				best = id
			}
		}
	}
	return best
}

// Deps returns the (sorted) ids the given resource depends on.
func (d *DAG) Deps(id string) []string { return d.deps[id] }

// Nodes returns the resource ids in IR (deterministic) order.
func (d *DAG) Nodes() []string { return append([]string{}, d.nodes...) }

// ApplyOrder returns ids in a deterministic dependency order (every resource
// after the ones it depends on) via Kahn's algorithm over the ready set.
func (d *DAG) ApplyOrder() []string {
	done := map[string]bool{}
	var order []string
	for len(order) < len(d.nodes) {
		ready := d.Ready(done)
		if len(ready) == 0 {
			break // cycle (Build already rejects these); avoid an infinite loop
		}
		for _, id := range ready {
			done[id] = true
			order = append(order, id)
		}
	}
	return order
}

// DestroyOrder returns ids in reverse dependency order: a dependent is destroyed
// before the resources it depends on. This is the reverse of ApplyOrder.
func (d *DAG) DestroyOrder() []string {
	order := d.ApplyOrder()
	rev := make([]string, len(order))
	for i, id := range order {
		rev[len(order)-1-i] = id
	}
	return rev
}

// Ready returns, in deterministic order, the resource ids whose dependencies are
// all satisfied (present in `done`) and which are not themselves done yet.
func (d *DAG) Ready(done map[string]bool) []string {
	var ready []string
	for _, id := range d.nodes {
		if done[id] {
			continue
		}
		ok := true
		for _, dep := range d.deps[id] {
			if !done[dep] {
				ok = false
				break
			}
		}
		if ok {
			ready = append(ready, id)
		}
	}
	return ready
}

// findCycle returns a cycle (list of ids) if the graph is cyclic, else nil.
func (d *DAG) findCycle() []string {
	const (
		white = 0 // unvisited
		gray  = 1 // on the current DFS stack
		black = 2 // fully explored
	)
	color := make(map[string]int, len(d.nodes))
	var stack []string
	var visit func(id string) []string
	visit = func(id string) []string {
		color[id] = gray
		stack = append(stack, id)
		for _, dep := range d.deps[id] {
			switch color[dep] {
			case white:
				if c := visit(dep); c != nil {
					return c
				}
			case gray:
				// Found a back-edge: extract the cycle from the stack.
				return extractCycle(stack, dep)
			}
		}
		stack = stack[:len(stack)-1]
		color[id] = black
		return nil
	}
	for _, id := range d.nodes {
		if color[id] == white {
			if c := visit(id); c != nil {
				return c
			}
		}
	}
	return nil
}

func extractCycle(stack []string, start string) []string {
	for i, id := range stack {
		if id == start {
			cyc := append([]string{}, stack[i:]...)
			return append(cyc, start) // close the loop for readability
		}
	}
	return append([]string{}, stack...)
}
