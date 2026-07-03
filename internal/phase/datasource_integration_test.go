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
	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/state"
)

// TestDataSourceFeedsResource proves the datasource path end to end against the
// REAL fake provider binary: a datasource (alpha_lookup) is READ, its computed
// output resolves a resource's __ref, and the resource applies with the
// datasource-derived value. The datasource is never planned, applied, or stored.
func TestDataSourceFeedsResource(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")

	// IR (constant across phases): a datasource with fully-known config, and a
	// resource whose `label` references the datasource's `result` output.
	ir := func(_ *ledger.Ledger) []byte {
		return []byte(fmt.Sprintf(`{
		  "schemaVersion":1,
		  "providers":{"alpha":{"source":%q,"config":{}}},
		  "resources":[
		    {"id":"alpha.alpha_token.web","provider":"alpha","type":"alpha_token","name":"web",
		     "config":{"label":{"__ref":{"resource":"data.alpha.alpha_lookup.q","path":["result"]}}}}
		  ],
		  "dataSources":[
		    {"id":"data.alpha.alpha_lookup.q","provider":"alpha","type":"alpha_lookup","name":"q",
		     "config":{"query":"vpc-1"}}
		  ],
		  "edges":[{"from":"data.alpha.alpha_lookup.q","to":"alpha.alpha_token.web","via":"label"}],
		  "nixConsumers":[]
		}`, alpha))
	}

	mgr := plugin.NewManager()
	defer mgr.Close()
	st, _ := state.Open(t.TempDir() + "/state.json")
	d := &phase.Driver{Eval: &phase.StubEvaluator{IRForLedger: ir}, Manager: mgr, Store: st, Ledger: ledger.New(), MaxPhases: 10}

	res, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// the datasource read produced result = "found:vpc-1", which fed the token's
	// label, so the token's computed value embeds it.
	if got := res.Outputs["data.alpha.alpha_lookup.q.result"]; got != "found:vpc-1" {
		t.Errorf("datasource result = %q, want %q", got, "found:vpc-1")
	}
	tokenVal := res.Outputs["alpha.alpha_token.web.value"]
	if !strings.Contains(tokenVal, "found:vpc-1") {
		t.Errorf("resource value = %q, want it to embed the datasource result %q", tokenVal, "found:vpc-1")
	}

	// the datasource was read, NOT written to state (only the resource is stored).
	if _, found, _ := st.Get("data.alpha.alpha_lookup.q"); found {
		t.Error("a datasource must not be written to state")
	}
	if _, found, _ := st.Get("alpha.alpha_token.web"); !found {
		t.Error("the resource should be in state")
	}
}

// TestDataSourceDependsOnResourceOutput proves the round-trip case: a datasource
// whose config depends on a resource's apply-time output reads in a LATER phase.
// alpha_token.A applies (phase 1) -> its value feeds the datasource's query
// (resolved by re-eval, phase 2) -> the datasource result feeds token B (phase 3).
func TestDataSourceDependsOnResourceOutput(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")

	// The stub models Nix re-eval: a __derived leaf becomes concrete once its
	// input is in the ledger (exactly like irChain does for resources).
	ir := func(l *ledger.Ledger) []byte {
		// datasource query = A.value once known, else __derived on it.
		query := derivedOrValue(l, "alpha.alpha_token.A", "value", func(v string) string { return v })
		// token B's label = the datasource result once known, else __derived.
		bLabel := derivedOrValue(l, "data.alpha.alpha_lookup.d", "result", func(v string) string { return v })
		return []byte(fmt.Sprintf(`{
		  "schemaVersion":1,
		  "providers":{"alpha":{"source":%q,"config":{}}},
		  "resources":[
		    {"id":"alpha.alpha_token.A","provider":"alpha","type":"alpha_token","name":"A","config":{}},
		    {"id":"alpha.alpha_token.B","provider":"alpha","type":"alpha_token","name":"B","config":{"label":%s}}
		  ],
		  "dataSources":[
		    {"id":"data.alpha.alpha_lookup.d","provider":"alpha","type":"alpha_lookup","name":"d","config":{"query":%s}}
		  ],
		  "edges":[],
		  "nixConsumers":[]
		}`, alpha, bLabel, query))
	}

	mgr := plugin.NewManager()
	defer mgr.Close()
	st, _ := state.Open(t.TempDir() + "/state.json")
	d := &phase.Driver{Eval: &phase.StubEvaluator{IRForLedger: ir}, Manager: mgr, Store: st, Ledger: ledger.New(), MaxPhases: 10}

	res, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// A applies first; its value (alpha::0 with no label) feeds the datasource
	// query; the datasource result feeds B. The chain forces 3 phases.
	if res.AppliedPhases != 3 {
		t.Errorf("applied phases = %d, want 3 (resource -> datasource -> resource chain)", res.AppliedPhases)
	}
	aVal := res.Outputs["alpha.alpha_token.A.value"]
	dResult := res.Outputs["data.alpha.alpha_lookup.d.result"]
	if dResult != "found:"+aVal {
		t.Errorf("datasource result = %q, want %q (derived from A.value)", dResult, "found:"+aVal)
	}
	if bVal := res.Outputs["alpha.alpha_token.B.value"]; !strings.Contains(bVal, dResult) {
		t.Errorf("B.value = %q, want it to embed the datasource result %q", bVal, dResult)
	}
}

// TestReplanDatasourceDependentReportsNoop is the regression guard for the two
// plan/apply output-fidelity bugs surfaced by the features-0.4 tutorial:
//   - beans-oh90: a resource whose config reads a datasource was reported `~ update`
//     on a re-plan, because PlanReport did not read the (unstored) datasource and
//     the resource was therefore not resolvable. It must now report OpNoop.
//   - beans-z57y: apply reported every node as a create. AppliedNode now carries the
//     real op, so a re-apply of an unchanged datasource-dependent resource reports
//     OpNoop (and the stored id/value are unchanged: a real no-op, not a re-create).
func TestReplanDatasourceDependentReportsNoop(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")

	// Same shape as the tutorial: a known-config datasource feeds a resource's label.
	ir := func(_ *ledger.Ledger) []byte {
		return []byte(fmt.Sprintf(`{
		  "schemaVersion":1,
		  "providers":{"alpha":{"source":%q,"config":{}}},
		  "resources":[
		    {"id":"alpha.alpha_token.web","provider":"alpha","type":"alpha_token","name":"web",
		     "config":{"label":{"__ref":{"resource":"data.alpha.alpha_lookup.q","path":["result"]}}}}
		  ],
		  "dataSources":[
		    {"id":"data.alpha.alpha_lookup.q","provider":"alpha","type":"alpha_lookup","name":"q",
		     "config":{"query":"vpc-1"}}
		  ],
		  "edges":[{"from":"data.alpha.alpha_lookup.q","to":"alpha.alpha_token.web","via":"label"}],
		  "nixConsumers":[]
		}`, alpha))
	}

	mgr := plugin.NewManager()
	defer mgr.Close()
	st, _ := state.Open(t.TempDir() + "/state.json")
	newDriver := func() *phase.Driver {
		return &phase.Driver{Eval: &phase.StubEvaluator{IRForLedger: ir}, Manager: mgr, Store: st, Ledger: ledger.New(), MaxPhases: 10}
	}

	// First apply: the resource is created.
	res1, err := newDriver().Run(context.Background())
	if err != nil {
		t.Fatalf("apply #1: %v", err)
	}
	if op := appliedOp(t, res1, "alpha.alpha_token.web"); op != plan.OpCreate {
		t.Errorf("apply #1: web op = %v, want OpCreate", op)
	}
	stored1, found, _ := st.Get("alpha.alpha_token.web")
	if !found {
		t.Fatal("resource not in state after apply #1")
	}

	// Re-PLAN: the datasource-dependent resource must report OpNoop, not OpUpdate
	// (the oh90 bug). PlanReport reads the datasource so the ref resolves.
	items, err := newDriver().PlanReport(context.Background())
	if err != nil {
		t.Fatalf("re-plan: %v", err)
	}
	if op := planOp(t, items, "alpha.alpha_token.web"); op != plan.OpNoop {
		t.Errorf("re-plan: web op = %v, want OpNoop (regression: datasource-dependent resource must not show ~ update)", op)
	}

	// Re-APPLY: the resource reports OpNoop (the z57y bug: not a create), and its
	// stored id/value are unchanged (a real no-op, not a re-create).
	res2, err := newDriver().Run(context.Background())
	if err != nil {
		t.Fatalf("apply #2: %v", err)
	}
	if op := appliedOp(t, res2, "alpha.alpha_token.web"); op != plan.OpNoop {
		t.Errorf("apply #2: web op = %v, want OpNoop (regression: re-apply must not report a create)", op)
	}
	stored2, _, _ := st.Get("alpha.alpha_token.web")
	if stored2.Attrs["id"] != stored1.Attrs["id"] || stored2.Attrs["value"] != stored1.Attrs["value"] {
		t.Errorf("re-apply changed the stored resource: id/value %v -> %v (must be unchanged on a no-op)", stored1.Attrs, stored2.Attrs)
	}
}

// appliedOp returns the op the apply result recorded for a resource id.
func appliedOp(t *testing.T, res *phase.Result, id string) plan.Op {
	t.Helper()
	for _, group := range res.Phases {
		for _, n := range group {
			if n.ID == id {
				return n.Op
			}
		}
	}
	t.Fatalf("id %q not found in the apply result phases", id)
	return plan.OpCreate
}

// planOp returns the op PlanReport assigned to a resource id.
func planOp(t *testing.T, items []phase.PlanItem, id string) plan.Op {
	t.Helper()
	for _, it := range items {
		if it.ID == id {
			return it.Op
		}
	}
	t.Fatalf("id %q not found in the plan report", id)
	return plan.OpCreate
}
