// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase_test

// DESIGN D3 decides what a phase costs:
//
//	TF→TF:  resolved INSIDE the executor during apply; no re-eval needed.
//	*→Nix:  requires re-eval with the value injected — this drives phase count.
//
// These tests pin the first half. A chain of plain references must resolve in
// ONE phase however deep it is, because nothing in it needs Nix to run again.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/phase"
)

// plainChainIR is A -> B -> C wired with plain __ref only. No __derived
// anywhere, so the stub returns the same IR every phase: there is nothing for a
// re-evaluation to resolve.
func plainChainIR(alphaBin string) func(*ledger.Ledger) []byte {
	return func(*ledger.Ledger) []byte {
		return []byte(fmt.Sprintf(`{
		  "schemaVersion":1,
		  "providers":{"alpha":{"source":%q,"config":{}}},
		  "resources":[
		    {"id":"alpha.alpha_token.A","provider":"alpha","type":"alpha_token","name":"A","config":{}},
		    {"id":"alpha.alpha_token.B","provider":"alpha","type":"alpha_token","name":"B",
		     "config":{"label":{"__ref":{"resource":"alpha.alpha_token.A","path":["value"]}}}},
		    {"id":"alpha.alpha_token.C","provider":"alpha","type":"alpha_token","name":"C",
		     "config":{"label":{"__ref":{"resource":"alpha.alpha_token.B","path":["value"]}}}}
		  ],
		  "edges":[],"nixConsumers":[]
		}`, alphaBin))
	}
}

// countingEval wraps an evaluator and counts how many times Nix would run.
type countingEval struct {
	inner phase.NixEvaluator
	calls int
}

func (c *countingEval) Eval(ctx context.Context, l *ledger.Ledger) ([]byte, error) {
	c.calls++
	return c.inner.Eval(ctx, l)
}

// A chain of plain references resolves in one phase, in dependency order.
func TestPlainRefChainResolvesInOnePhase(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")

	d, closer := newDriver(t, &phase.StubEvaluator{IRForLedger: plainChainIR(alpha)})
	defer closer()

	res, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if res.AppliedPhases != 1 {
		t.Errorf("applied phases = %d, want 1: every hop is a plain __ref, and DESIGN D3 "+
			"says a TF→TF reference needs no re-evaluation. Each extra phase is a full "+
			"`nix eval` that resolves nothing.", res.AppliedPhases)
	}

	// Order is the other half of the contract: resolving earlier must not mean
	// resolving out of dependency order.
	want := []string{"alpha.alpha_token.A", "alpha.alpha_token.B", "alpha.alpha_token.C"}
	if len(res.Applied) != len(want) {
		t.Fatalf("applied = %v, want %v", res.Applied, want)
	}
	for i := range want {
		if res.Applied[i] != want[i] {
			t.Errorf("applied[%d] = %q, want %q (dependency order must hold)",
				i, res.Applied[i], want[i])
		}
	}

	// And the phase's group must list everything it resolved, including the
	// nodes that only became ready as it progressed.
	if len(res.Phases) != 1 || len(res.Phases[0]) != 3 {
		t.Errorf("phase groups = %v, want one group of three", res.Phases)
	}
}

// The saving is the point: one evaluation, not one per link.
func TestPlainRefChainEvaluatesOnce(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")

	counter := &countingEval{inner: &phase.StubEvaluator{IRForLedger: plainChainIR(alpha)}}
	d, closer := newDriver(t, counter)
	defer closer()

	if _, err := d.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}

	if counter.calls != 1 {
		t.Errorf("evaluations = %d, want 1 for a chain that needs no re-evaluation", counter.calls)
	}
}

// A datasource sits in the same readiness list as a resource, so it must follow
// the same rule: reading it needs no re-evaluation when its inputs are already
// known. A resource -> datasource -> resource chain wired with plain references
// therefore resolves in ONE phase.
func TestPlainRefDatasourceChainResolvesInOnePhase(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")

	irFn := func(*ledger.Ledger) []byte {
		return []byte(fmt.Sprintf(`{
		  "schemaVersion":1,
		  "providers":{"alpha":{"source":%q,"config":{}}},
		  "resources":[
		    {"id":"alpha.alpha_token.A","provider":"alpha","type":"alpha_token","name":"A","config":{}},
		    {"id":"alpha.alpha_token.B","provider":"alpha","type":"alpha_token","name":"B",
		     "config":{"label":{"__ref":{"resource":"data.alpha.alpha_lookup.q","path":["result"]}}}}
		  ],
		  "dataSources":[
		    {"id":"data.alpha.alpha_lookup.q","provider":"alpha","type":"alpha_lookup","name":"q",
		     "config":{"query":{"__ref":{"resource":"alpha.alpha_token.A","path":["value"]}}}}
		  ],
		  "edges":[],"nixConsumers":[]
		}`, alpha))
	}

	d, closer := newDriver(t, &phase.StubEvaluator{IRForLedger: irFn})
	defer closer()

	res, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if res.AppliedPhases != 1 {
		t.Errorf("applied phases = %d, want 1: the resource, the datasource read and the "+
			"consuming resource are all plain references", res.AppliedPhases)
	}
	want := []string{"alpha.alpha_token.A", "data.alpha.alpha_lookup.q", "alpha.alpha_token.B"}
	if len(res.Applied) != len(want) {
		t.Fatalf("resolved = %v, want %v", res.Applied, want)
	}
	for i := range want {
		if res.Applied[i] != want[i] {
			t.Errorf("resolved[%d] = %q, want %q", i, res.Applied[i], want[i])
		}
	}
	// The datasource must still be reported as a READ, not an apply.
	var sawRead bool
	for _, n := range res.Phases[0] {
		if n.ID == "data.alpha.alpha_lookup.q" {
			sawRead = n.IsData
		}
	}
	if !sawRead {
		t.Error("the datasource should be reported as a read, not an apply")
	}
}

// A phase that can make no progress at all is still stuck: resolving within the
// phase must not turn an unresolvable graph into an infinite loop.
func TestUnresolvableChainIsStillStuck(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")

	irFn := func(*ledger.Ledger) []byte {
		return []byte(fmt.Sprintf(`{
		  "schemaVersion":1,
		  "providers":{"alpha":{"source":%q,"config":{}}},
		  "resources":[
		    {"id":"alpha.alpha_token.X","provider":"alpha","type":"alpha_token","name":"X",
		     "config":{"label":{"__derived":{"inputs":["alpha.alpha_token.never.value"]}}}}
		  ],
		  "edges":[],"nixConsumers":[]
		}`, alpha))
	}

	d, closer := newDriver(t, &phase.StubEvaluator{IRForLedger: irFn})
	defer closer()

	_, err := d.Run(context.Background())
	if err == nil {
		t.Fatal("a node whose input never resolves must be reported as stuck")
	}
	if !strings.Contains(err.Error(), "alpha.alpha_token.X") {
		t.Errorf("the error should name the stuck node; got %v", err)
	}
}
