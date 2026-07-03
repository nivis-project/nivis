// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/phase"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/state"
)

// TestRealNixRoundTrip is the headline mechanism end-to-end: the REAL Nix flake
// (.#nivis.plan) re-evaluated each phase with the accumulating ledger, driving
// the REAL fake provider binaries through plan/apply, to a fixpoint. This is the
// round trip the project exists to prove (DESIGN D3 / docs/TESTING.md).
//
// It exercises the full example topology:
//
//	A (alpha)              -> A.value
//	B (beta) from=rec-+A.value          (__derived on A.value)
//	C (alpha) label=B.endpoint::A.value (__derived on both)
//	systemConfig consumer reads from BOTH providers.
func TestRealNixRoundTrip(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not on PATH")
	}
	root := repoRoot(t)
	// The flake's provider sources are bare names ("provider-<x>"); put the built
	// fakes on $PATH so the executor's exec.Command resolves them (as `nix shell
	// .#fake-providers` would).
	buildFakesOnPath(t, root)

	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")

	mgr := plugin.NewManager()
	defer mgr.Close()
	st, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))

	d := &phase.Driver{
		Eval:      phase.NixEval{FlakeRef: ".", Attr: "nivis.plan", WorkDir: root},
		Manager:   mgr,
		Store:     st,
		Ledger:    ledger.New(),
		MaxPhases: 10,
	}

	res, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}

	// ≥3 apply phases (the Nix-mediated chain forces N>2).
	if res.AppliedPhases != 3 {
		t.Errorf("applied phases = %d, want 3", res.AppliedPhases)
	}
	// Final ledger has all three resources' computed outputs.
	for _, want := range []string{
		"alpha.alpha_token.A.value",
		"beta.beta_record.B.endpoint",
		"alpha.alpha_token.C.value",
	} {
		if _, ok := res.Outputs[want]; !ok {
			t.Errorf("final ledger missing %s; have %v", want, keys(res.Outputs))
		}
	}
	// The TF->Nix round trip: B.endpoint derives from A.value through a Nix string.
	// A has no label so A.value = "alpha::0"; B.from = "rec-alpha::0";
	// B.endpoint = "beta://rec-alpha::0".
	if got := res.Outputs["beta.beta_record.B.endpoint"]; got != "beta://rec-alpha::0" {
		t.Errorf("B.endpoint = %q, want beta://rec-alpha::0", got)
	}

	// The nixConsumer reading from BOTH providers must be fully concrete at
	// fixpoint (no remaining placeholders).
	var consumer map[string]interface{}
	for _, c := range res.LastIR.Consumers {
		if c.ID == "systemConfig" {
			consumer = c.Value
		}
	}
	if consumer == nil {
		t.Fatal("systemConfig consumer missing from final IR")
	}
	for _, k := range []string{"recordEndpoint", "tokenValue", "combined"} {
		v := consumer[k]
		if _, isStr := v.(string); !isStr {
			t.Errorf("consumer.%s not concrete at fixpoint: %#v", k, v)
		}
	}
	if consumer["combined"] != "beta://rec-alpha::0::alpha::0" {
		t.Errorf("consumer.combined = %v, want beta://rec-alpha::0::alpha::0", consumer["combined"])
	}
}

// buildFakesOnPath builds the fake providers into a temp dir and prepends it to
// $PATH, so a bare-name provider `source` (e.g. "provider-alpha", as the example
// flake configs use) resolves via the executor's exec.Command PATH lookup, just
// as `nix shell .#fake-providers` provides them. Returns nothing; the providers
// are found on PATH.
func buildFakesOnPath(t *testing.T, root string) {
	t.Helper()
	dir := t.TempDir()
	for _, pkg := range []string{"provider-alpha", "provider-beta"} {
		cmd := exec.Command("go", "build", "-o", filepath.Join(dir, pkg), "./cmd/"+pkg)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("cannot build %s: %v\n%s", pkg, err, out)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
