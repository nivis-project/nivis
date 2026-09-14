// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase_test

// The driver's progress-event stream. These assert on the FACTS the driver
// reports — what started, what it was, what it resolved as — and never on
// wording, because the driver composes none.

import (
	"context"
	"testing"

	"github.com/nivis-project/nivis/internal/phase"
	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/progress"
)

// rec collects the driver's events.
type rec struct{ events []progress.Event }

func (r *rec) Emit(e progress.Event) { r.events = append(r.events, e) }

func (r *rec) ofKind(k progress.Kind) []progress.Event {
	var out []progress.Event
	for _, e := range r.events {
		if e.Kind == k {
			out = append(out, e)
		}
	}
	return out
}

// Every phase reports its evaluation, and every evaluation that starts also
// finishes: a renderer pairs them to show elapsed time, and an unpaired start
// would leave a spinner running forever.
func TestEvalEventsArePairedPerPhase(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")
	beta := buildProvider(t, "provider-beta")

	d, closer := newDriver(t, &phase.StubEvaluator{IRForLedger: irChain(alpha, beta)})
	defer closer()
	r := &rec{}
	d.Observer = r

	res, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	starts := r.ofKind(progress.EvalStart)
	dones := r.ofKind(progress.EvalDone)
	if len(starts) != len(dones) {
		t.Errorf("eval starts = %d, dones = %d; every start must finish", len(starts), len(dones))
	}
	// One evaluation per phase the loop ran. The loop evaluates once more than it
	// applies, to confirm the fixpoint.
	if len(starts) < res.AppliedPhases {
		t.Errorf("eval starts = %d, want at least one per applied phase (%d)",
			len(starts), res.AppliedPhases)
	}
	for _, e := range dones {
		if e.Err != nil {
			t.Errorf("phase %d eval reported an error on a successful run: %v", e.Phase, e.Err)
		}
	}
}

// A phase reports how many nodes are ready BEFORE it runs them. That count is
// the only honest denominator available: the run's grand total cannot be known
// in advance, because a later phase can reveal resources phase 0 cannot see.
func TestPhaseStartCountsTheNodesItWillResolve(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")
	beta := buildProvider(t, "provider-beta")

	d, closer := newDriver(t, &phase.StubEvaluator{IRForLedger: irChain(alpha, beta)})
	defer closer()
	r := &rec{}
	d.Observer = r

	res, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	phases := r.ofKind(progress.PhaseStart)
	if len(phases) != res.AppliedPhases {
		t.Fatalf("PhaseStart events = %d, want one per applied phase (%d)",
			len(phases), res.AppliedPhases)
	}
	// The A -> B -> C chain resolves exactly one node per phase.
	for i, e := range phases {
		if e.Count != len(res.Phases[i]) {
			t.Errorf("phase %d announced %d node(s) but resolved %d",
				i, e.Count, len(res.Phases[i]))
		}
	}
}

// Each node is bracketed, and the done event carries what a renderer needs: the
// op it actually resolved as, and the provider-assigned id where there is one.
func TestNodeEventsCarryOpAndResourceID(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")
	beta := buildProvider(t, "provider-beta")

	d, closer := newDriver(t, &phase.StubEvaluator{IRForLedger: irChain(alpha, beta)})
	defer closer()
	r := &rec{}
	d.Observer = r

	if _, err := d.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}

	starts := r.ofKind(progress.NodeStart)
	dones := r.ofKind(progress.NodeDone)
	if len(starts) != 3 || len(dones) != 3 {
		t.Fatalf("node starts = %d, dones = %d; want 3 each (A, B, C)", len(starts), len(dones))
	}
	for _, e := range dones {
		if e.Err != nil {
			t.Errorf("%s reported an error on a successful run: %v", e.ID, e.Err)
		}
		if e.ID == "" || e.Type == "" {
			t.Errorf("a node event must identify itself; got id=%q type=%q", e.ID, e.Type)
		}
		if e.IsData {
			t.Errorf("%s is a resource, not a datasource", e.ID)
		}
		// Every resource here is new, so each is a create.
		if e.Op != plan.OpCreate {
			t.Errorf("%s op = %v, want create", e.ID, e.Op)
		}
		if e.Duration <= 0 {
			t.Errorf("%s carries no duration; a renderer cannot report elapsed time", e.ID)
		}
	}
}

// A Driver with no Observer must behave exactly as it did before events existed.
// This is what keeps every existing caller and test unaffected.
func TestDriverWithoutObserverIsUnchanged(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")
	beta := buildProvider(t, "provider-beta")

	d, closer := newDriver(t, &phase.StubEvaluator{IRForLedger: irChain(alpha, beta)})
	defer closer()
	// No d.Observer set.

	res, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if res.AppliedPhases != 3 {
		t.Errorf("applied phases = %d, want 3", res.AppliedPhases)
	}
}
