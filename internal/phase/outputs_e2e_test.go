// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/phase"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/state"
)

// TestStackOutputsResolveE2E is the end-to-end for declared stack outputs (A7):
// the REAL Nix flake (nix/example/default.nix declares `outputs`) applied against
// the REAL fake provider binaries, then ResolveOutputs returns the concrete
// values. It proves outputs ride the same phased resolution as the round trip,
// including a value composed across BOTH providers.
func TestStackOutputsResolveE2E(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not on PATH")
	}
	root := repoRoot(t)
	buildFakesOnPath(t, root)
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")

	mgr := plugin.NewManager()
	defer mgr.Close()
	st, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))

	newDriver := func() *phase.Driver {
		return &phase.Driver{
			Eval:      phase.NixEval{FlakeRef: ".", Attr: "nivis.plan", WorkDir: root},
			Manager:   mgr,
			Store:     st,
			Ledger:    ledger.New(),
			MaxPhases: 10,
		}
	}

	// Apply to a fixpoint (resources land in state).
	if _, err := newDriver().Run(context.Background()); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Now resolve the declared outputs against state (a fresh driver, as a
	// standalone `nivis output` invocation would).
	outs, err := newDriver().ResolveOutputs(context.Background())
	if err != nil {
		t.Fatalf("ResolveOutputs: %v", err)
	}

	// `token` = A.value (single resource, from alpha). A has no label => "alpha::0".
	if got := outs["token"]; got != "alpha::0" {
		t.Errorf("output token = %#v, want \"alpha::0\"", got)
	}
	// `combined` = B.endpoint :: A.value (composed across BOTH providers, resolved
	// across phases): "beta://rec-alpha::0" :: "alpha::0".
	if got := outs["combined"]; got != "beta://rec-alpha::0::alpha::0" {
		t.Errorf("output combined = %#v, want \"beta://rec-alpha::0::alpha::0\"", got)
	}
	// every declared output is concrete (a string), no placeholders.
	for name, v := range outs {
		if _, ok := v.(string); !ok {
			t.Errorf("output %q is not concrete: %#v", name, v)
		}
	}
}

// TestStackOutputsResolveDatasource is the regression guard for an output that
// references a DATASOURCE result. Datasources are not persisted to state, so a
// standalone `nivis output` (ResolveOutputs) must re-read them to resolve such an
// output, not leave it as a raw __ref. Uses the nivis.tutorial flake attr (which
// declares a datasource-derived output) with a required --var.
func TestStackOutputsResolveDatasource(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not on PATH")
	}
	root := repoRoot(t)
	buildFakesOnPath(t, root)
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")

	mgr := plugin.NewManager()
	defer mgr.Close()
	st, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))

	newDriver := func() *phase.Driver {
		l := ledger.New()
		l.Vars = map[string]interface{}{"env": "prod"} // the tutorial's required var
		return &phase.Driver{
			Eval:      phase.NixEval{FlakeRef: ".", Attr: "nivis.tutorial", WorkDir: root},
			Manager:   mgr,
			Store:     st,
			Ledger:    l,
			MaxPhases: 10,
		}
	}

	if _, err := newDriver().Run(context.Background()); err != nil {
		t.Fatalf("apply tutorial: %v", err)
	}
	outs, err := newDriver().ResolveOutputs(context.Background())
	if err != nil {
		t.Fatalf("ResolveOutputs: %v", err)
	}
	// lookupResult comes from the alpha_lookup datasource (result = "found:<query>",
	// query = env = "prod"). It must be the concrete value, not a __ref.
	if got := outs["lookupResult"]; got != "found:prod" {
		t.Errorf("datasource-derived output lookupResult = %#v, want \"found:prod\" (a datasource output must re-read, not stay a __ref)", got)
	}
}
