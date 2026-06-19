// Package e2e holds the milestone exit test: the headline two-provider round
// trip (docs/TESTING.md). It drives the REAL flake and REAL fake provider
// binaries through the phase driver — no stubs, no network.
package e2e_test

import (
	"context"
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

func requireNix(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not on PATH")
	}
}

// buildBinaries builds the fake providers into a temp dir and prepends it to
// $PATH, so the bare-name provider `source` in the example flake configs resolves
// via the executor's exec.Command PATH lookup (as `nix shell .#fake-providers`
// would provide them).
func buildBinaries(t *testing.T, root string) {
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

func newDriver(t *testing.T, root, attr string, maxPhases int) (*phase.Driver, func()) {
	t.Helper()
	mgr := plugin.NewManager()
	st, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))
	d := &phase.Driver{
		Eval:      phase.NixEval{FlakeRef: ".", Attr: attr, WorkDir: root},
		Manager:   mgr,
		Store:     st,
		Ledger:    ledger.New(),
		MaxPhases: maxPhases,
	}
	return d, mgr.Close
}

// TestHeadlineRoundTrip is the milestone exit criterion: two providers, unknowns
// on both sides, ≥3 phases to fixpoint, a both-providers Nix consumer concrete.
func TestHeadlineRoundTrip(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")

	d, closer := newDriver(t, root, "nivis.plan", 10)
	defer closer()

	res, err := d.Run(context.Background())
	if err != nil {
		t.Fatalf("headline run: %v", err)
	}

	// (1) exactly 3 apply phases, halting at fixpoint (driver halts when all
	// resources applied; not a hardcoded count).
	if res.AppliedPhases != 3 {
		t.Errorf("applied phases = %d, want 3", res.AppliedPhases)
	}

	// (2) final ledger contains A.id, A.value, B.endpoint, C.* (id + value).
	for _, k := range []string{
		"alpha.alpha_token.A.id",
		"alpha.alpha_token.A.value",
		"beta.beta_record.B.endpoint",
		"alpha.alpha_token.C.id",
		"alpha.alpha_token.C.value",
	} {
		if _, ok := res.Outputs[k]; !ok {
			t.Errorf("final ledger missing %s", k)
		}
	}

	// (3) systemConfig consumer concrete from BOTH providers, exact values.
	consumer := consumerValue(t, res, "systemConfig")
	// A has no label -> A.value = "alpha::0"; B.from = "rec-alpha::0";
	// B.endpoint = "beta://rec-alpha::0"; combined = endpoint + "::" + A.value.
	want := map[string]string{
		"recordEndpoint": "beta://rec-alpha::0",
		"tokenValue":     "alpha::0",
		"combined":       "beta://rec-alpha::0::alpha::0",
	}
	for k, w := range want {
		got, ok := consumer[k].(string)
		if !ok {
			t.Errorf("consumer.%s not concrete: %#v", k, consumer[k])
			continue
		}
		if got != w {
			t.Errorf("consumer.%s = %q, want %q", k, got, w)
		}
	}
}

// TestTwoPhaseCapInsufficient proves N>2 is required: a 2-phase cap cannot
// resolve the headline topology.
func TestTwoPhaseCapInsufficient(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")

	d, closer := newDriver(t, root, "nivis.plan", 2)
	defer closer()

	res, err := d.Run(context.Background())
	if err == nil {
		t.Fatalf("2-phase cap must NOT resolve a 3-phase chain; got success: %+v", res)
	}
	// C must be among the unresolved (it needs phase 3).
	if !strings.Contains(err.Error(), "exceeded") {
		t.Fatalf("expected phase-cap error, got: %v", err)
	}
}

// TestCycleRejected: the cyclic variant halts at fixpoint naming A and C.
func TestCycleRejected(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")

	d, closer := newDriver(t, root, "nivis.planCycle", 10)
	defer closer()

	_, err := d.Run(context.Background())
	if err == nil {
		t.Fatal("cyclic plan must be rejected at fixpoint")
	}
	msg := err.Error()
	if !strings.Contains(msg, "alpha.alpha_token.A") || !strings.Contains(msg, "alpha.alpha_token.C") {
		t.Fatalf("cycle error must name A and C; got: %v", err)
	}
	if !strings.Contains(msg, "unresolvable") {
		t.Fatalf("cycle error should describe unresolvability; got: %v", err)
	}
}

func consumerValue(t *testing.T, res *phase.Result, id string) map[string]interface{} {
	t.Helper()
	for _, c := range res.LastIR.Consumers {
		if c.ID == id {
			return c.Value
		}
	}
	t.Fatalf("consumer %q not in final IR", id)
	return nil
}
