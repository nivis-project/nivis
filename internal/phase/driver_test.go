// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/wearetechnative/nivis/internal/ledger"
	"github.com/wearetechnative/nivis/internal/phase"
	"github.com/wearetechnative/nivis/internal/plugin"
	"github.com/wearetechnative/nivis/internal/state"
)

// These tests stub the Nix-eval step (StubEvaluator) but use the REAL plugin
// manager + REAL fake provider binaries + REAL state, so the loop, fixpoint,
// stuck detection, and plan/apply path are all exercised for real. Only Nix
// re-evaluation is modeled, by emitting phase-appropriate IR from the ledger.

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		p := filepath.Dir(dir)
		if p == dir {
			t.Fatal("go.mod not found")
		}
		dir = p
	}
}

func buildProvider(t *testing.T, pkg string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), pkg)
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/"+pkg)
	cmd.Dir = repoRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot build %s: %v\n%s", pkg, err, out)
	}
	return bin
}

// irChain emits IR for the A -> B(derived on A.value) -> C(derived on B.endpoint)
// topology, resolving derived leaves from the ledger exactly as Nix would on
// re-eval: once an input is in the ledger, that derived leaf becomes concrete.
func irChain(alphaBin, betaBin string) func(l *ledger.Ledger) []byte {
	return func(l *ledger.Ledger) []byte {
		// B.from is "rec-"+A.value once A.value known, else __derived.
		bFrom := derivedOrValue(l, "alpha.alpha_token.A", "value", func(v string) string { return "rec-" + v })
		// C.label is B.endpoint once known, else __derived on it.
		cLabel := derivedOrValue(l, "beta.beta_record.B", "endpoint", func(v string) string { return v })
		return []byte(fmt.Sprintf(`{
		  "schemaVersion":1,
		  "providers":{
		    "alpha":{"source":%q,"config":{}},
		    "beta":{"source":%q,"config":{}}
		  },
		  "resources":[
		    {"id":"alpha.alpha_token.A","provider":"alpha","type":"alpha_token","name":"A","config":{}},
		    {"id":"beta.beta_record.B","provider":"beta","type":"beta_record","name":"B","config":{"from":%s}},
		    {"id":"alpha.alpha_token.C","provider":"alpha","type":"alpha_token","name":"C","config":{"label":%s}}
		  ],
		  "edges":[],
		  "nixConsumers":[]
		}`, alphaBin, betaBin, bFrom, cLabel))
	}
}

// derivedOrValue returns a concrete JSON string value if <id>.<attr> is in the
// ledger (after applying transform), else a __derived leaf naming the input.
func derivedOrValue(l *ledger.Ledger, id, attr string, transform func(string) string) string {
	if l.Known(id, attr) {
		v := l.Outputs[id][attr].(string)
		return fmt.Sprintf("%q", transform(v))
	}
	return fmt.Sprintf(`{"__derived":{"inputs":[%q]}}`, id+"."+attr)
}

func newDriver(t *testing.T, eval phase.NixEvaluator) (*phase.Driver, func()) {
	t.Helper()
	mgr := plugin.NewManager()
	st, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))
	d := &phase.Driver{
		Eval:    eval,
		Manager: mgr,
		Store:   st,
		Ledger:  ledger.New(),
	}
	return d, mgr.Close
}

func TestThreePhaseChain(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")
	beta := buildProvider(t, "provider-beta")

	d, closer := newDriver(t, &phase.StubEvaluator{IRForLedger: irChain(alpha, beta)})
	defer closer()

	res, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// A applies phase 1, B phase 2 (after re-eval resolves rec-...), C phase 3.
	if res.AppliedPhases != 3 {
		t.Fatalf("applied phases = %d, want 3 (chain is Nix-mediated, forcing N>2)", res.AppliedPhases)
	}
	want := []string{"alpha.alpha_token.A", "beta.beta_record.B", "alpha.alpha_token.C"}
	if strings.Join(res.Applied, ",") != strings.Join(want, ",") {
		t.Fatalf("apply order = %v, want %v", res.Applied, want)
	}
	// B.from was "rec-"+A.value; A.value = "alpha::0" (no label, counter 0).
	if got := res.Outputs["beta.beta_record.B.endpoint"]; got != "beta://rec-alpha::0" {
		t.Errorf("B.endpoint = %q, want beta://rec-alpha::0", got)
	}

	// Per-phase grouping (A3): 3 groups, one id each, concatenation == Applied,
	// none marked as a datasource read (all three are resources).
	if len(res.Phases) != 3 {
		t.Fatalf("Phases groups = %d, want 3", len(res.Phases))
	}
	var flat []string
	for _, group := range res.Phases {
		for _, n := range group {
			if n.IsData {
				t.Errorf("%s wrongly marked as a datasource read", n.ID)
			}
			flat = append(flat, n.ID)
		}
	}
	if strings.Join(flat, ",") != strings.Join(res.Applied, ",") {
		t.Errorf("Phases concatenation %v != Applied %v", flat, res.Applied)
	}
}

func TestTwoPhaseCapLeavesPending(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")
	beta := buildProvider(t, "provider-beta")

	d, closer := newDriver(t, &phase.StubEvaluator{IRForLedger: irChain(alpha, beta)})
	defer closer()
	d.MaxPhases = 2 // cap below the 3 the chain requires

	_, err := d.Run(context.Background())
	if err == nil {
		t.Fatal("a 2-phase cap must NOT resolve a 3-phase chain (proves N>2 required)")
	}
	if !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("expected a phase-cap error, got: %v", err)
	}
}

func TestStuckResourceNamed(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	alpha := buildProvider(t, "provider-alpha")

	// B depends (derived) on a producer that never appears -> stuck at fixpoint.
	stub := func(l *ledger.Ledger) []byte {
		return []byte(fmt.Sprintf(`{
		  "schemaVersion":1,
		  "providers":{"alpha":{"source":%q,"config":{}}},
		  "resources":[
		    {"id":"alpha.alpha_token.A","provider":"alpha","type":"alpha_token","name":"A","config":{}},
		    {"id":"alpha.alpha_token.B","provider":"alpha","type":"alpha_token","name":"B",
		     "config":{"label":{"__derived":{"inputs":["alpha.alpha_token.ghost.value"]}}}}
		  ],
		  "edges":[],"nixConsumers":[]
		}`, alpha))
	}
	d, closer := newDriver(t, &phase.StubEvaluator{IRForLedger: stub})
	defer closer()

	_, err := d.Run(context.Background())
	if err == nil {
		t.Fatal("expected a stuck/fixpoint error")
	}
	if !strings.Contains(err.Error(), "alpha.alpha_token.B") || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("stuck error should name B and the awaited ghost input; got: %v", err)
	}
}

func TestLedgerSaved0600(t *testing.T) {
	l := ledger.New()
	l.Append("a.t.A", map[string]interface{}{"v": "x"})
	path := filepath.Join(t.TempDir(), "ledger.json")
	if err := l.Save(path); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("ledger mode = %o, want 600", fi.Mode().Perm())
	}
	got, err := ledger.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Known("a.t.A", "v") {
		t.Error("ledger did not round-trip")
	}
}
