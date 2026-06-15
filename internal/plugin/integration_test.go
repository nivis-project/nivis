package plugin_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/wearetechnative/nixform/internal/apply"
	"github.com/wearetechnative/nixform/internal/ir"
	"github.com/wearetechnative/nixform/internal/plan"
	"github.com/wearetechnative/nixform/internal/plugin"
	"github.com/wearetechnative/nixform/internal/state"
)

// buildProvider compiles a fake provider binary into a temp dir and returns its
// path. Skips the test (not fail) if the toolchain can't build it.
func buildProvider(t *testing.T, pkg string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), filepath.Base(pkg))
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/"+pkg)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("cannot build %s (%v): %s", pkg, err, out)
	}
	return bin
}

// repoRoot walks up from the test's working dir to the module root (where go.mod
// lives), so `go build ./cmd/...` resolves regardless of test cwd.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod")
		}
		dir = parent
	}
}

func node(id, typ, name string, cfg map[string]interface{}) *ir.ResourceNode {
	return &ir.ResourceNode{Resource: ir.Resource{ID: id, Type: typ, Name: name, Config: cfg}}
}

func TestAlphaEndToEnd(t *testing.T) {
	bin := buildProvider(t, "provider-alpha")
	t.Setenv("NIXFORM_FAKE_COUNTER", "") // deterministic seed 0

	mgr := plugin.NewManager()
	defer mgr.Close()
	ctx := context.Background()

	client, err := mgr.Client("alpha", bin)
	if err != nil {
		t.Fatalf("spawn/handshake: %v", err)
	}

	rs, err := plan.SchemaFor(ctx, client, "alpha_token")
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	n := node("alpha.alpha_token.A", "alpha_token", "A", map[string]interface{}{"label": "rec-X"})

	pr, err := plan.Plan(ctx, client, rs, n, n.Resource.Config)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	// computed id/value must be unknown at plan
	if !contains(pr.UnknownAfterApply, "id") || !contains(pr.UnknownAfterApply, "value") {
		t.Fatalf("plan should mark id+value unknown-after-apply; got %v", pr.UnknownAfterApply)
	}

	st, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))
	attrs, err := apply.Apply(ctx, client, rs, n, n.Resource.Config, pr.PlannedState, st)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if attrs["id"] != "alpha-0" {
		t.Errorf("id = %v, want alpha-0", attrs["id"])
	}
	if attrs["value"] != "alpha:rec-X:0" {
		t.Errorf("value = %v, want alpha:rec-X:0", attrs["value"])
	}

	// persisted to state
	got, ok, err := st.Get("alpha.alpha_token.A")
	if err != nil || !ok {
		t.Fatalf("state get: ok=%v err=%v", ok, err)
	}
	if got.Attrs["value"] != "alpha:rec-X:0" {
		t.Errorf("persisted value = %v, want alpha:rec-X:0", got.Attrs["value"])
	}
}

func TestBetaEndToEnd(t *testing.T) {
	bin := buildProvider(t, "provider-beta")
	mgr := plugin.NewManager()
	defer mgr.Close()
	ctx := context.Background()

	client, err := mgr.Client("beta", bin)
	if err != nil {
		t.Fatalf("spawn/handshake: %v", err)
	}
	rs, err := plan.SchemaFor(ctx, client, "beta_record")
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	n := node("beta.beta_record.B", "beta_record", "B", map[string]interface{}{"from": "alpha:rec-X:0"})

	pr, err := plan.Plan(ctx, client, rs, n, n.Resource.Config)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !contains(pr.UnknownAfterApply, "endpoint") {
		t.Fatalf("endpoint should be unknown-after-apply; got %v", pr.UnknownAfterApply)
	}

	st, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))
	attrs, err := apply.Apply(ctx, client, rs, n, n.Resource.Config, pr.PlannedState, st)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if attrs["endpoint"] != "beta://alpha:rec-X:0" {
		t.Errorf("endpoint = %v, want beta://alpha:rec-X:0", attrs["endpoint"])
	}
}

func TestPooledByIdentity(t *testing.T) {
	bin := buildProvider(t, "provider-alpha")
	mgr := plugin.NewManager()
	defer mgr.Close()
	c1, err := mgr.Client("alpha", bin)
	if err != nil {
		t.Fatal(err)
	}
	c2, err := mgr.Client("alpha", bin)
	if err != nil {
		t.Fatal(err)
	}
	if c1 != c2 {
		t.Error("expected the same pooled client for one identity")
	}
}

func contains(ss []string, x string) bool {
	for _, s := range ss {
		if s == x {
			return true
		}
	}
	return false
}
