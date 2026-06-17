// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/wearetechnative/nivis/internal/ledger"
	"github.com/wearetechnative/nivis/internal/phase"
	"github.com/wearetechnative/nivis/internal/plugin"
	"github.com/wearetechnative/nivis/internal/state"
)

// TestVarFlowsThroughPhaseIntoConfig proves the end-to-end variable path: a
// resolved variable in the ledger's `vars` reaches the (stubbed) Nix eval, is
// embedded into a resource's config, and the REAL fake provider applies it, so
// the computed output reflects the variable's value.
//
// The Driver's ledger is pre-seeded with Vars (modeling cmd/nivis newLedger,
// which resolves --var/--var-file/env once). The StubEvaluator reads
// l.Vars["label"] and emits the resource config with that value, modeling what
// nivis.mkVars does in real Nix (vars.label -> the resource's label). The
// alpha_token fake computes value = "alpha:<label>:<n>", an observable proof the
// variable flowed ledger -> eval -> config -> apply.
func TestVarFlowsThroughPhaseIntoConfig(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")

	stub := &phase.StubEvaluator{
		IRForLedger: func(l *ledger.Ledger) []byte {
			// model mkVars: read the injected variable; require it present.
			label, ok := l.Vars["label"].(string)
			if !ok {
				label = "<unset>"
			}
			return []byte(fmt.Sprintf(`{
			  "schemaVersion":1,
			  "providers":{"alpha":{"source":%q,"config":{}}},
			  "resources":[
			    {"id":"alpha.alpha_token.V","provider":"alpha","type":"alpha_token","name":"V","config":{"label":%q}}
			  ],
			  "edges":[],
			  "nixConsumers":[]
			}`, alpha, label))
		},
	}

	mgr := plugin.NewManager()
	defer mgr.Close()
	st, _ := state.Open(t.TempDir() + "/state.json")
	// pre-seed Vars, as the CLI's newLedger() does after resolving --var.
	l := ledger.New()
	l.Vars = map[string]interface{}{"label": "prod-eu"}
	d := &phase.Driver{Eval: stub, Manager: mgr, Store: st, Ledger: l}

	res, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// the fake computes value = "alpha:<label>:<n>"; the var-supplied label must
	// appear, proving it flowed all the way into the applied resource.
	got, ok := res.Outputs["alpha.alpha_token.V.value"]
	if !ok {
		t.Fatalf("no value output; have %v", res.Outputs)
	}
	if !strings.Contains(got, "prod-eu") {
		t.Errorf("computed value = %q, want it to contain the variable value %q", got, "prod-eu")
	}
}

// TestVarsConstantAcrossPhases: vars are injected unchanged on every phase (the
// stub records what it saw each call).
func TestVarsConstantAcrossPhases(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")
	beta := buildProvider(t, "provider-beta")

	var seen []string
	stub := &phase.StubEvaluator{
		IRForLedger: func(l *ledger.Ledger) []byte {
			if v, ok := l.Vars["label"].(string); ok {
				seen = append(seen, v)
			} else {
				seen = append(seen, "<absent>")
			}
			// reuse the multi-phase chain so there is more than one phase.
			return irChain(alpha, beta)(l)
		},
	}

	mgr := plugin.NewManager()
	defer mgr.Close()
	st, _ := state.Open(t.TempDir() + "/state.json")
	l := ledger.New()
	l.Vars = map[string]interface{}{"label": "const"}
	d := &phase.Driver{Eval: stub, Manager: mgr, Store: st, Ledger: l, MaxPhases: 10}

	if _, err := d.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(seen) < 2 {
		t.Fatalf("expected more than one phase, saw %d evals", len(seen))
	}
	for i, v := range seen {
		if v != "const" {
			t.Errorf("phase %d saw vars.label=%q, want it constant as %q", i, v, "const")
		}
	}
}
