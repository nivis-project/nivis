// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/phase"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/state"
)

// chainIR is the topology from bean nixform2-nwnf, three layers deep: the middle
// resource is replaced while the one above it still holds its output.
//
//	bottom <- middle <- top
//
// `image` is force-new on the middle resource, so changing it forces a
// replacement whose destroy would orphan `top`.
func chainIR(bin, image string) func(*ledger.Ledger) []byte {
	return func(_ *ledger.Ledger) []byte {
		return []byte(fmt.Sprintf(`{
		  "schemaVersion":1,
		  "providers":{"delta":{"source":%q,"config":{}}},
		  "resources":[
		    {"id":"delta.delta_thing.bottom","provider":"delta","type":"delta_thing","name":"bottom",
		     "config":{"label":"b"}},
		    {"id":"delta.delta_thing.middle","provider":"delta","type":"delta_thing","name":"middle",
		     "config":{"label":{"__ref":{"resource":"delta.delta_thing.bottom","path":["id"]}},"image":%q}},
		    {"id":"delta.delta_thing.top","provider":"delta","type":"delta_thing","name":"top",
		     "config":{"label":{"__ref":{"resource":"delta.delta_thing.middle","path":["id"]}}}}
		  ],
		  "dataSources":[],
		  "edges":[],
		  "nixConsumers":[]
		}`, bin, image))
	}
}

func chainDriver(t *testing.T, bin, image, statePath string) (*phase.Driver, func()) {
	t.Helper()
	st, err := state.Open(statePath)
	if err != nil {
		t.Fatal(err)
	}
	mgr := plugin.NewManager()
	return &phase.Driver{
		Eval:      &phase.StubEvaluator{IRForLedger: chainIR(bin, image)},
		Manager:   mgr,
		Store:     st,
		Ledger:    ledger.New(),
		MaxPhases: 10,
	}, mgr.Close
}

// applyChain brings the three-layer chain up and returns the state path.
func applyChain(t *testing.T, bin string) string {
	t.Helper()
	statePath := t.TempDir() + "/state.json"
	d, closeMgr := chainDriver(t, bin, "ami-old", statePath)
	defer closeMgr()
	if _, err := d.Run(context.Background()); err != nil {
		t.Fatalf("initial apply: %v", err)
	}
	return statePath
}

// TestReplacementHeldByDependentsIsRefused is the regression for bean
// nixform2-hpmb, and the topology is the one from nixform2-nwnf that no test
// covered: replacing the middle of a three-layer chain.
func TestReplacementHeldByDependentsIsRefused(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath := applyChain(t, bin)

	before, _ := state.Open(statePath)
	mid, found, _ := before.Get("delta.delta_thing.middle")
	if !found {
		t.Fatalf("expected the middle resource in state after the first apply")
	}

	d, closeMgr := chainDriver(t, bin, "ami-new", statePath)
	_, err := d.Run(context.Background())
	closeMgr()

	if err == nil {
		t.Fatalf("expected the replacement to be refused while top holds middle")
	}
	msg := err.Error()
	for _, want := range []string{"delta.delta_thing.middle", "delta.delta_thing.top", "depend"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not mention %q: %s", want, msg)
		}
	}

	// Nothing was destroyed: the middle resource's prior object is intact and
	// state is unchanged.
	after, _ := state.Open(statePath)
	got, found, _ := after.Get("delta.delta_thing.middle")
	if !found {
		t.Fatalf("the middle resource was destroyed despite the refusal")
	}
	if got.Attrs["id"] != mid.Attrs["id"] {
		t.Errorf("middle id = %v, was %v: it was replaced despite the refusal", got.Attrs["id"], mid.Attrs["id"])
	}
}

// A resource nothing holds still replaces normally.
func TestReplacementWithNoDependentsStillProceeds(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath := applyChain(t, bin)

	// `top` is the end of the chain, so nothing depends on it. Replace it by
	// pointing the IR at a changed image for top instead of middle.
	ir := func(_ *ledger.Ledger) []byte {
		return []byte(fmt.Sprintf(`{
		  "schemaVersion":1,
		  "providers":{"delta":{"source":%q,"config":{}}},
		  "resources":[
		    {"id":"delta.delta_thing.bottom","provider":"delta","type":"delta_thing","name":"bottom",
		     "config":{"label":"b"}},
		    {"id":"delta.delta_thing.middle","provider":"delta","type":"delta_thing","name":"middle",
		     "config":{"label":{"__ref":{"resource":"delta.delta_thing.bottom","path":["id"]}},"image":"ami-old"}},
		    {"id":"delta.delta_thing.top","provider":"delta","type":"delta_thing","name":"top",
		     "config":{"label":{"__ref":{"resource":"delta.delta_thing.middle","path":["id"]}},"image":"ami-new"}}
		  ],
		  "dataSources":[],
		  "edges":[],
		  "nixConsumers":[]
		}`, bin))
	}
	st, _ := state.Open(statePath)
	mgr := plugin.NewManager()
	d := &phase.Driver{
		Eval: &phase.StubEvaluator{IRForLedger: ir}, Manager: mgr, Store: st,
		Ledger: ledger.New(), MaxPhases: 10,
	}
	_, err := d.Run(context.Background())
	mgr.Close()
	if err != nil {
		t.Fatalf("replacing a resource nothing depends on must proceed: %v", err)
	}

	after, _ := state.Open(statePath)
	rs, found, _ := after.Get("delta.delta_thing.top")
	if !found {
		t.Fatalf("expected top in state")
	}
	if rs.Attrs["image"] != "ami-new" {
		t.Errorf("top image = %v, want ami-new", rs.Attrs["image"])
	}
}

// A dependent declared in the configuration but never applied holds nothing, so
// it must not block. Presence in STATE is the test, not presence in the graph.
func TestDependentAbsentFromStateDoesNotBlock(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath := t.TempDir() + "/state.json"

	// Apply only the bottom two, so `top` exists in the configuration but not in
	// state when middle is replaced.
	twoLayer := func(image string) func(*ledger.Ledger) []byte {
		return func(_ *ledger.Ledger) []byte {
			return []byte(fmt.Sprintf(`{
			  "schemaVersion":1,
			  "providers":{"delta":{"source":%q,"config":{}}},
			  "resources":[
			    {"id":"delta.delta_thing.bottom","provider":"delta","type":"delta_thing","name":"bottom",
			     "config":{"label":"b"}},
			    {"id":"delta.delta_thing.middle","provider":"delta","type":"delta_thing","name":"middle",
			     "config":{"label":{"__ref":{"resource":"delta.delta_thing.bottom","path":["id"]}},"image":%q}}
			  ],
			  "dataSources":[],
			  "edges":[],
			  "nixConsumers":[]
			}`, bin, image))
		}
	}
	st, _ := state.Open(statePath)
	mgr := plugin.NewManager()
	d := &phase.Driver{
		Eval: &phase.StubEvaluator{IRForLedger: twoLayer("ami-old")}, Manager: mgr,
		Store: st, Ledger: ledger.New(), MaxPhases: 10,
	}
	if _, err := d.Run(context.Background()); err != nil {
		mgr.Close()
		t.Fatalf("initial apply: %v", err)
	}
	mgr.Close()

	// Now the configuration gains `top`, and middle is replaced in the same run.
	// `top` is not in state yet, so it holds nothing.
	d2, closeMgr2 := chainDriver(t, bin, "ami-new", statePath)
	_, err := d2.Run(context.Background())
	closeMgr2()
	if err != nil {
		t.Fatalf("a dependent absent from state must not block the replacement: %v", err)
	}
}

// The plan reports the blockage without changing anything.
func TestPlanReportsAHeldReplacement(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath := applyChain(t, bin)

	d, closeMgr := chainDriver(t, bin, "ami-new", statePath)
	items, err := d.PlanReport(context.Background())
	closeMgr()
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	var found bool
	for _, it := range items {
		if it.ID != "delta.delta_thing.middle" {
			continue
		}
		found = true
		if len(it.HeldBy) != 1 || it.HeldBy[0] != "delta.delta_thing.top" {
			t.Errorf("HeldBy = %v, want [delta.delta_thing.top]", it.HeldBy)
		}
	}
	if !found {
		t.Fatalf("the middle resource was not reported: %#v", items)
	}

	// Side-effect free: state is untouched by a plan.
	after, _ := state.Open(statePath)
	if _, ok, _ := after.Get("delta.delta_thing.middle"); !ok {
		t.Errorf("the plan changed state")
	}
}
