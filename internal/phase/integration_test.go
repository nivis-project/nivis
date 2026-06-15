// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/wearetechnative/terrae-nivis/internal/ledger"
	"github.com/wearetechnative/terrae-nivis/internal/phase"
	"github.com/wearetechnative/terrae-nivis/internal/plugin"
	"github.com/wearetechnative/terrae-nivis/internal/provider"
	"github.com/wearetechnative/terrae-nivis/internal/state"
)

// TestRealNixRoundTrip is the headline mechanism end-to-end: the REAL Nix flake
// (.#terraeNivis.plan) re-evaluated each phase with the accumulating ledger, driving
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
	// The flake's provider sources are "./bin/provider-<x>"; build them there so
	// the spawned paths resolve relative to the repo (the executor uses the
	// provider 'source' field verbatim).
	buildInto(t, root, "provider-alpha")
	buildInto(t, root, "provider-beta")

	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")

	mgr := plugin.NewManager()
	defer mgr.Close()
	st, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))

	d := &phase.Driver{
		Eval:      phase.NixEval{FlakeRef: ".", Attr: "terraeNivis.plan", WorkDir: root},
		Manager:   relativeManager{mgr: mgr, root: root},
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

func buildInto(t *testing.T, root, pkg string) {
	t.Helper()
	bin := filepath.Join(root, "bin", pkg)
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/"+pkg)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot build %s: %v\n%s", pkg, err, out)
	}
}

// relativeManager resolves a provider 'source' like "./bin/provider-alpha"
// against the repo root before spawning, then delegates to the real manager.
type relativeManager struct {
	mgr  *plugin.Manager
	root string
}

func (m relativeManager) Client(identity, path string, config map[string]interface{}) (provider.Client, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.root, path)
	}
	return m.mgr.Client(identity, path, config)
}

func keys(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
