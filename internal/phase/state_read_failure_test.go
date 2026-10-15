// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/phase"
	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/state"
)

// errListStore is a state store whose whole-document read fails, as a denied
// credential or an unreachable bucket would. Everything else delegates to a
// real store, so only the failure under test is synthetic.
type errListStore struct {
	state.Store
	err error
}

func (s errListStore) List() ([]state.ResourceState, error) { return nil, s.err }

var errBackendDown = errors.New("state: read \"s3://bucket/state.json\": AccessDenied")

func failingStore(t *testing.T) state.Store {
	t.Helper()
	inner, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatal(err)
	}
	return errListStore{Store: inner, err: errBackendDown}
}

// TestPlanReportSurfacesAStateReadFailure is the regression for bean
// nixform2-j02d. Reporting "there is no state" when the truth is "I cannot read
// your state" turns a backend outage into a plan that proposes creating
// everything.
func TestPlanReportSurfacesAStateReadFailure(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	mgr := plugin.NewManager()
	defer mgr.Close()

	d := &phase.Driver{
		Eval:    &phase.StubEvaluator{IRForLedger: chainIR(bin, "ami-old")},
		Manager: mgr, Store: failingStore(t), Ledger: ledger.New(), MaxPhases: 10,
	}

	items, err := d.PlanReport(context.Background())
	if err == nil {
		t.Fatalf("expected the backend failure to surface; got %d items instead", len(items))
	}
	if !errors.Is(err, errBackendDown) {
		t.Errorf("error does not wrap the backend failure: %v", err)
	}
	if !strings.Contains(err.Error(), "state") {
		t.Errorf("error does not say the failure was reading state: %v", err)
	}
}

func TestResolveOutputsSurfacesAStateReadFailure(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	mgr := plugin.NewManager()
	defer mgr.Close()

	d := &phase.Driver{
		Eval:    &phase.StubEvaluator{IRForLedger: chainIR(bin, "ami-old")},
		Manager: mgr, Store: failingStore(t), Ledger: ledger.New(), MaxPhases: 10,
	}

	if _, err := d.ResolveOutputs(context.Background()); err == nil {
		t.Fatalf("expected the backend failure to surface")
	} else if !errors.Is(err, errBackendDown) {
		t.Errorf("error does not wrap the backend failure: %v", err)
	}
}

// switchableStore delegates to a real store until listErr is set, so a run can
// get under way and fail the read at the moment the guard needs it.
type switchableStore struct {
	state.Store
	listErr error
}

func (s *switchableStore) List() ([]state.ResourceState, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.Store.List()
}

// TestReplacementRefusedWhenStateCannotBeRead is the destructive half. The guard
// added in refuse-a-replacement-that-orphans-dependents concluded "nothing
// depends on it" when it could not read state, which removed the guard at
// exactly the moment it mattered.
func TestReplacementRefusedWhenStateCannotBeRead(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	statePath := applyChain(t, bin)

	before, _ := state.Open(statePath)
	mid, found, _ := before.Get("delta.delta_thing.middle")
	if !found {
		t.Fatalf("expected the middle resource in state after the first apply")
	}

	inner, err := state.Open(statePath)
	if err != nil {
		t.Fatal(err)
	}
	st := &switchableStore{Store: inner}

	mgr := plugin.NewManager()
	d := &phase.Driver{
		Eval:    &phase.StubEvaluator{IRForLedger: chainIR(bin, "ami-new")},
		Manager: mgr, Store: st, Ledger: ledger.New(), MaxPhases: 10,
	}
	// The read fails only once the run is under way, so the resource is reached
	// and its replacement is actually attempted.
	st.listErr = errBackendDown

	_, runErr := d.Run(context.Background())
	mgr.Close()

	if runErr == nil {
		t.Fatalf("expected the replacement to be refused when state cannot be read")
	}
	if !strings.Contains(runErr.Error(), "dependents could not be determined") {
		t.Errorf("error does not say why it refused: %v", runErr)
	}

	// The prior object must be intact: the guard failed closed.
	after, _ := state.Open(statePath)
	got, stillThere, _ := after.Get("delta.delta_thing.middle")
	if !stillThere {
		t.Fatalf("the middle resource was destroyed despite the refusal")
	}
	if got.Attrs["id"] != mid.Attrs["id"] {
		t.Errorf("middle id = %v, was %v: it was replaced despite the refusal", got.Attrs["id"], mid.Attrs["id"])
	}
}

// A first run has no state document. The store returns that as an empty
// document rather than an error, so propagating must not break it. This is the
// regression the change could plausibly cause.
func TestFirstRunWithNoStateFileStillReportsCreates(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	bin := buildProvider(t, "provider-delta")
	mgr := plugin.NewManager()
	defer mgr.Close()

	st, err := state.Open(t.TempDir() + "/never-written.json")
	if err != nil {
		t.Fatal(err)
	}
	d := &phase.Driver{
		Eval:    &phase.StubEvaluator{IRForLedger: chainIR(bin, "ami-old")},
		Manager: mgr, Store: st, Ledger: ledger.New(), MaxPhases: 10,
	}

	items, err := d.PlanReport(context.Background())
	if err != nil {
		t.Fatalf("a first run must not fail because no state document exists: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected the resources to be reported")
	}
	for _, it := range items {
		if it.Op != plan.OpCreate {
			t.Errorf("%s reported as %v, want create on a first run", it.ID, it.Op)
		}
	}
}
