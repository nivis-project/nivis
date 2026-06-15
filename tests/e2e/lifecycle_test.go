package e2e_test

import (
	"context"
	"testing"

	"github.com/wearetechnative/nivis/internal/destroy"
	"github.com/wearetechnative/nivis/internal/ir"
	"github.com/wearetechnative/nivis/internal/ledger"
	"github.com/wearetechnative/nivis/internal/phase"
	"github.com/wearetechnative/nivis/internal/plugin"
	"github.com/wearetechnative/nivis/internal/refresh"
	"github.com/wearetechnative/nivis/internal/state"
)

// TestLifecycleRefreshThenDestroy completes the headline-e2e lifecycle
// assertions deferred from E4b: after a full apply, refresh leaves the converged
// state unchanged, and destroy removes C, B, A in reverse dependency order.
func TestLifecycleRefreshThenDestroy(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")

	statePath := t.TempDir() + "/state.json"
	st, _ := state.Open(statePath)
	mgr := plugin.NewManager()
	defer mgr.Close()
	rmgr := relMgr{mgr: mgr, root: root}
	ctx := context.Background()

	// 1. Apply the headline topology to a fixpoint.
	d := &phase.Driver{
		Eval:      phase.NixEval{FlakeRef: ".", Attr: "nivis.plan", WorkDir: root},
		Manager:   rmgr,
		Store:     st,
		Ledger:    ledger.New(),
		MaxPhases: 10,
	}
	if _, err := d.Run(ctx); err != nil {
		t.Fatalf("apply: %v", err)
	}
	before := snapshot(t, st)
	if len(before) != 3 {
		t.Fatalf("expected 3 resources in state after apply, got %d", len(before))
	}

	// 2. Ingest the phase-0 (unresolved) graph for destroy/refresh: the
	//    dependency structure (incl. Nix-mediated derived deps) lives there;
	//    once resolved, derived leaves are concrete and carry no dep info.
	g := phase0Graph(t, ctx, root)

	// 3. Refresh: state must be unchanged (fake ReadResource echoes state).
	if _, err := refresh.Run(ctx, g, rmgr, st); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	after := snapshot(t, st)
	for id, attrs := range before {
		for k, v := range attrs {
			if after[id][k] != v {
				t.Errorf("refresh changed %s.%s: %v -> %v", id, k, v, after[id][k])
			}
		}
	}

	// 4. Destroy: must remove C, B, A in reverse dependency order.
	res, err := destroy.Run(ctx, g, rmgr, st, destroy.Options{})
	if err != nil {
		t.Fatalf("destroy: %v", err)
	}
	want := []string{"alpha.alpha_token.C", "beta.beta_record.B", "alpha.alpha_token.A"}
	if len(res.Destroyed) != 3 {
		t.Fatalf("destroyed %v, want 3 in order %v", res.Destroyed, want)
	}
	for i := range want {
		if res.Destroyed[i] != want[i] {
			t.Errorf("destroy order[%d] = %s, want %s", i, res.Destroyed[i], want[i])
		}
	}
	if items, _ := st.List(); len(items) != 0 {
		t.Errorf("state not empty after destroy: %d remain", len(items))
	}
}

// phase0Graph evaluates the plan with an empty ledger so the dependency
// structure (including Nix-mediated __derived deps) is present for destroy/
// refresh ordering. Destroy/refresh use stored state for values, not config.
func phase0Graph(t *testing.T, ctx context.Context, root string) *ir.Graph {
	t.Helper()
	ev := phase.NixEval{FlakeRef: ".", Attr: "nivis.plan", WorkDir: root}
	irJSON, err := ev.Eval(ctx, ledger.New())
	if err != nil {
		t.Fatalf("phase-0 eval: %v", err)
	}
	g, err := ir.IngestIR(irJSON)
	if err != nil {
		t.Fatalf("phase-0 ingest: %v", err)
	}
	return g
}

func snapshot(t *testing.T, st state.Store) map[string]map[string]interface{} {
	t.Helper()
	items, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]interface{}{}
	for _, rs := range items {
		out[rs.ID] = rs.Attrs
	}
	return out
}
