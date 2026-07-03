// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package destroy_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nivis-project/nivis/internal/destroy"
	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/state"
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

func buildAlpha(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "provider-alpha")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/provider-alpha")
	cmd.Dir = repoRoot(t)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot build provider-alpha: %v\n%s", err, out)
	}
	return bin
}

// graphAB builds an A <- B graph (B depends on A) with the given alpha binary.
func graphAB(t *testing.T, bin string, preventDestroyB bool) *ir.Graph {
	t.Helper()
	meta := ""
	if preventDestroyB {
		meta = `,"meta":{"lifecycle":{"preventDestroy":true}}`
	}
	s := `{
	  "schemaVersion":1,"providers":{"alpha":{"source":"` + bin + `","config":{}}},
	  "resources":[
	    {"id":"alpha.alpha_token.A","provider":"alpha","type":"alpha_token","name":"A","config":{}},
	    {"id":"alpha.alpha_token.B","provider":"alpha","type":"alpha_token","name":"B",
	     "config":{"label":{"__ref":{"resource":"alpha.alpha_token.A","path":["value"]}}}` + meta + `}
	  ],"edges":[],"nixConsumers":[]}`
	g, err := ir.IngestIR([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func seedState(t *testing.T) (state.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	st, _ := state.Open(path)
	for _, id := range []string{"alpha.alpha_token.A", "alpha.alpha_token.B"} {
		if err := st.Set(state.ResourceState{ID: id, Type: "alpha_token", Attrs: map[string]interface{}{"id": "x", "value": "v"}}); err != nil {
			t.Fatal(err)
		}
	}
	return st, path
}

func TestDestroyReverseOrder(t *testing.T) {
	bin := buildAlpha(t)
	g := graphAB(t, bin, false)
	st, _ := seedState(t)
	mgr := plugin.NewManager()
	defer mgr.Close()

	res, err := destroy.Run(context.Background(), g, mgr, st, destroy.Options{})
	if err != nil {
		t.Fatalf("destroy: %v", err)
	}
	// B depends on A, so destroy order is B then A.
	want := []string{"alpha.alpha_token.B", "alpha.alpha_token.A"}
	if len(res.Destroyed) != 2 || res.Destroyed[0] != want[0] || res.Destroyed[1] != want[1] {
		t.Fatalf("destroyed = %v, want %v (reverse dep order)", res.Destroyed, want)
	}
	// State emptied.
	if items, _ := st.List(); len(items) != 0 {
		t.Errorf("state should be empty after destroy, has %d", len(items))
	}
}

func TestDestroyHonorsPreventDestroy(t *testing.T) {
	bin := buildAlpha(t)
	g := graphAB(t, bin, true) // B has preventDestroy
	st, _ := seedState(t)
	mgr := plugin.NewManager()
	defer mgr.Close()

	_, err := destroy.Run(context.Background(), g, mgr, st, destroy.Options{})
	if err == nil {
		t.Fatal("destroy must refuse a preventDestroy resource")
	}
	// B is destroyed first (reverse order); it has preventDestroy -> error names B,
	// and nothing should have been removed.
	if items, _ := st.List(); len(items) != 2 {
		t.Errorf("no resource should be destroyed when B is protected; state has %d", len(items))
	}
}

func TestDestroyTarget(t *testing.T) {
	bin := buildAlpha(t)
	g := graphAB(t, bin, false)
	st, _ := seedState(t)
	mgr := plugin.NewManager()
	defer mgr.Close()

	res, err := destroy.Run(context.Background(), g, mgr, st, destroy.Options{Target: "alpha.alpha_token.B"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Destroyed) != 1 || res.Destroyed[0] != "alpha.alpha_token.B" {
		t.Fatalf("targeted destroy = %v, want only B", res.Destroyed)
	}
	if _, ok, _ := st.Get("alpha.alpha_token.A"); !ok {
		t.Error("A should remain when only B is targeted")
	}
}
