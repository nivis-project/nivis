// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package graph_test

import (
	"reflect"
	"testing"

	"github.com/nivis-project/nivis/internal/graph"
	"github.com/nivis-project/nivis/internal/ir"
)

// chainGraph builds a graph from "dependent -> dependency" pairs, with nodes in
// the given order so the deterministic ordering of Dependents is assertable.
func chainGraph(t *testing.T, order []string, deps map[string][]string) *graph.DAG {
	t.Helper()
	g := &ir.Graph{Order: order, Nodes: map[string]*ir.ResourceNode{}}
	for _, id := range order {
		n := &ir.ResourceNode{Resource: ir.Resource{ID: id, Provider: "p", Type: "t", Name: id}}
		for _, d := range deps[id] {
			n.Refs = append(n.Refs, ir.RefEdge{Owner: id, Class: ir.ClassTFTF, Target: d})
		}
		g.Nodes[id] = n
	}
	d, err := graph.Build(g)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return d
}

func TestDependentsDirect(t *testing.T) {
	d := chainGraph(t, []string{"a", "b"}, map[string][]string{"b": {"a"}})

	if got := d.Dependents("a"); !reflect.DeepEqual(got, []string{"b"}) {
		t.Errorf("Dependents(a) = %v, want [b]", got)
	}
	if got := d.Dependents("b"); len(got) != 0 {
		t.Errorf("Dependents(b) = %v, want none", got)
	}
}

// The chain from bean nixform2-nwnf: replacing the middle node is held by
// everything above it, not only its direct dependent.
func TestDependentsTransitive(t *testing.T) {
	d := chainGraph(t, []string{"s3", "snapshot", "ami", "instance"}, map[string][]string{
		"snapshot": {"s3"},
		"ami":      {"snapshot"},
		"instance": {"ami"},
	})

	if got := d.Dependents("snapshot"); !reflect.DeepEqual(got, []string{"ami", "instance"}) {
		t.Errorf("Dependents(snapshot) = %v, want [ami instance]", got)
	}
}

// A node reached by two paths is reported once.
func TestDependentsDiamondReportsEachOnce(t *testing.T) {
	d := chainGraph(t, []string{"base", "left", "right", "top"}, map[string][]string{
		"left":  {"base"},
		"right": {"base"},
		"top":   {"left", "right"},
	})

	if got := d.Dependents("base"); !reflect.DeepEqual(got, []string{"left", "right", "top"}) {
		t.Errorf("Dependents(base) = %v, want [left right top] exactly once each", got)
	}
}

func TestDependentsNone(t *testing.T) {
	d := chainGraph(t, []string{"a", "b"}, map[string][]string{"b": {"a"}})

	if got := d.Dependents("missing"); len(got) != 0 {
		t.Errorf("Dependents of an unknown id = %v, want none", got)
	}
}

// Order follows the IR, so a message built from this is stable across runs.
func TestDependentsIsDeterministic(t *testing.T) {
	d := chainGraph(t, []string{"a", "z", "m"}, map[string][]string{
		"z": {"a"},
		"m": {"a"},
	})

	want := []string{"z", "m"} // IR order, not alphabetical
	for i := range 5 {
		if got := d.Dependents("a"); !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: Dependents(a) = %v, want %v", i, got, want)
		}
	}
}

// A __derived leaf is a real ordering dependency, so a holder reached only
// through a Nix-computed value is still reported.
func TestDependentsIncludesDerivedHolders(t *testing.T) {
	g := &ir.Graph{Order: []string{"a", "b"}, Nodes: map[string]*ir.ResourceNode{
		"a": {Resource: ir.Resource{ID: "a", Provider: "p", Type: "t", Name: "a"}},
		"b": {
			Resource: ir.Resource{ID: "b", Provider: "p", Type: "t", Name: "b"},
			Refs:     []ir.RefEdge{{Owner: "b", Class: ir.ClassStarToNix, Inputs: []string{"a.id"}}},
		},
	}}
	d, err := graph.Build(g)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if got := d.Dependents("a"); !reflect.DeepEqual(got, []string{"b"}) {
		t.Errorf("Dependents(a) = %v, want [b]: a derived leaf is an ordering dependency", got)
	}
}
