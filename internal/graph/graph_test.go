// Copyright 2026 WeareTechnative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package graph_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/wearetechnative/nivis/internal/graph"
	"github.com/wearetechnative/nivis/internal/ir"
)

func mustIngest(t *testing.T, s string) *ir.Graph {
	t.Helper()
	g, err := ir.IngestIR([]byte(s))
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	return g
}

// A (no deps) -> B (ref A.value). C derived on B (no executor dep from derived).
const chain = `{
  "schemaVersion":1,
  "providers":{"a":{"source":"x","config":{}}},
  "resources":[
    {"id":"a.t.A","provider":"a","type":"t","name":"A","config":{}},
    {"id":"a.t.B","provider":"a","type":"t","name":"B","config":{"in":{"__ref":{"resource":"a.t.A","path":["value"]}}}},
    {"id":"a.t.C","provider":"a","type":"t","name":"C","config":{"in":{"__derived":{"inputs":["a.t.B.out"]}}}}
  ],
  "edges":[],"nixConsumers":[]
}`

func TestReadyTopological(t *testing.T) {
	g := mustIngest(t, chain)
	d, err := graph.Build(g)
	if err != nil {
		t.Fatal(err)
	}
	// Nothing done: A and C are ready (C's only dep is a *->Nix derived, which
	// is NOT an executor dependency). B depends on A so it is not ready.
	ready := d.Ready(map[string]bool{})
	if !contains(ready, "a.t.A") {
		t.Errorf("A should be ready initially; got %v", ready)
	}
	if contains(ready, "a.t.B") {
		t.Errorf("B should NOT be ready before A; got %v", ready)
	}
	// After A done, B becomes ready.
	ready = d.Ready(map[string]bool{"a.t.A": true})
	if !contains(ready, "a.t.B") {
		t.Errorf("B should be ready after A; got %v", ready)
	}
}

func TestDependsOnRespected(t *testing.T) {
	const s = `{
	  "schemaVersion":1,"providers":{"a":{"source":"x","config":{}}},
	  "resources":[
	    {"id":"a.t.A","provider":"a","type":"t","name":"A","config":{}},
	    {"id":"a.t.B","provider":"a","type":"t","name":"B","config":{},"meta":{"dependsOn":["a.t.A"]}}
	  ],"edges":[],"nixConsumers":[]}`
	d, err := graph.Build(mustIngest(t, s))
	if err != nil {
		t.Fatal(err)
	}
	if contains(d.Ready(map[string]bool{}), "a.t.B") {
		t.Error("B must wait on depends_on A")
	}
	if !contains(d.Ready(map[string]bool{"a.t.A": true}), "a.t.B") {
		t.Error("B should be ready once A is done")
	}
}

func TestCycleDetected(t *testing.T) {
	const s = `{
	  "schemaVersion":1,"providers":{"a":{"source":"x","config":{}}},
	  "resources":[
	    {"id":"a.t.A","provider":"a","type":"t","name":"A","config":{"x":{"__ref":{"resource":"a.t.B","path":["v"]}}}},
	    {"id":"a.t.B","provider":"a","type":"t","name":"B","config":{"x":{"__ref":{"resource":"a.t.A","path":["v"]}}}}
	  ],"edges":[],"nixConsumers":[]}`
	_, err := graph.Build(mustIngest(t, s))
	if err == nil {
		t.Fatal("expected a cycle error")
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("error %q should mention cycle", err)
	}
	if !strings.Contains(err.Error(), "a.t.A") || !strings.Contains(err.Error(), "a.t.B") {
		t.Fatalf("cycle error %q should name A and B", err)
	}
}

func TestResolveTFTF(t *testing.T) {
	g := mustIngest(t, chain)
	out := graph.Outputs{"a.t.A": {"value": "x"}}
	res := graph.ResolveTFTF(g, out)

	// B.config.in was __ref(A.value) -> now "x", B fully known.
	if got := res.Configs["a.t.B"]["in"]; got != "x" {
		t.Errorf("B.in = %v, want x", got)
	}
	if !contains(res.FullyKnown, "a.t.B") {
		t.Errorf("B should be fully known; FullyKnown=%v", res.FullyKnown)
	}
	// C has a __derived leaf -> stays pending, leaf untouched.
	if !contains(res.Pending, "a.t.C") {
		t.Errorf("C should be pending (derived); Pending=%v", res.Pending)
	}
	if _, isDerived := res.Configs["a.t.C"]["in"].(map[string]interface{}); !isDerived {
		t.Errorf("C.in should remain a derived leaf, got %T", res.Configs["a.t.C"]["in"])
	}
	// A unchanged & known.
	if !contains(res.FullyKnown, "a.t.A") {
		t.Errorf("A should be fully known")
	}
}

func TestResolveTFTFPendingWhenOutputMissing(t *testing.T) {
	g := mustIngest(t, chain)
	res := graph.ResolveTFTF(g, graph.Outputs{}) // no outputs known
	if !contains(res.Pending, "a.t.B") {
		t.Errorf("B should be pending without A's output; Pending=%v", res.Pending)
	}
	// the ref leaf is preserved unresolved.
	leaf, ok := res.Configs["a.t.B"]["in"].(map[string]interface{})
	if !ok || !hasKey(leaf, "__ref") {
		t.Errorf("B.in should remain a __ref leaf, got %v", res.Configs["a.t.B"]["in"])
	}
}

func TestResolveNestedPath(t *testing.T) {
	const s = `{
	  "schemaVersion":1,"providers":{"a":{"source":"x","config":{}}},
	  "resources":[
	    {"id":"a.t.N","provider":"a","type":"t","name":"N","config":{}},
	    {"id":"a.t.U","provider":"a","type":"t","name":"U",
	     "config":{"ip":{"__ref":{"resource":"a.t.N","path":["net",0,"ip"]}}}}
	  ],"edges":[],"nixConsumers":[]}`
	g := mustIngest(t, s)
	out := graph.Outputs{"a.t.N": {"net": []interface{}{map[string]interface{}{"ip": "10.0.0.1"}}}}
	res := graph.ResolveTFTF(g, out)
	if got := res.Configs["a.t.U"]["ip"]; got != "10.0.0.1" {
		t.Errorf("U.ip = %v, want 10.0.0.1", got)
	}
}

func TestApplyAndDestroyOrder(t *testing.T) {
	// A <- B <- C  (B depends on A, C depends on B)
	const s = `{
	  "schemaVersion":1,"providers":{"a":{"source":"x","config":{}}},
	  "resources":[
	    {"id":"a.t.A","provider":"a","type":"t","name":"A","config":{}},
	    {"id":"a.t.B","provider":"a","type":"t","name":"B","config":{"in":{"__ref":{"resource":"a.t.A","path":["v"]}}}},
	    {"id":"a.t.C","provider":"a","type":"t","name":"C","config":{"in":{"__ref":{"resource":"a.t.B","path":["v"]}}}}
	  ],"edges":[],"nixConsumers":[]}`
	d, err := graph.Build(mustIngest(t, s))
	if err != nil {
		t.Fatal(err)
	}
	apply := d.ApplyOrder()
	if strings.Join(apply, ",") != "a.t.A,a.t.B,a.t.C" {
		t.Fatalf("apply order = %v, want A,B,C", apply)
	}
	destroy := d.DestroyOrder()
	if strings.Join(destroy, ",") != "a.t.C,a.t.B,a.t.A" {
		t.Fatalf("destroy order = %v, want C,B,A (reverse)", destroy)
	}
}

func contains(ss []string, x string) bool {
	for _, s := range ss {
		if s == x {
			return true
		}
	}
	return false
}

func hasKey(m map[string]interface{}, k string) bool { _, ok := m[k]; return ok }

var _ = reflect.DeepEqual
