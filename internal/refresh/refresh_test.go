// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

package refresh_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/wearetechnative/terrae-nivis/internal/ir"
	"github.com/wearetechnative/terrae-nivis/internal/plugin"
	"github.com/wearetechnative/terrae-nivis/internal/refresh"
	"github.com/wearetechnative/terrae-nivis/internal/state"
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

// TestRefreshNoChange: the fake provider's ReadResource echoes current state, so
// refresh on a converged store leaves every resource's attrs unchanged (and
// performs no apply).
func TestRefreshNoChange(t *testing.T) {
	bin := buildAlpha(t)
	g, err := ir.IngestIR([]byte(`{
	  "schemaVersion":1,"providers":{"alpha":{"source":"` + bin + `","config":{}}},
	  "resources":[{"id":"alpha.alpha_token.A","provider":"alpha","type":"alpha_token","name":"A","config":{}}],
	  "edges":[],"nixConsumers":[]}`))
	if err != nil {
		t.Fatal(err)
	}

	st, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))
	orig := map[string]interface{}{"id": "alpha-0", "value": "alpha::0"}
	if err := st.Set(state.ResourceState{ID: "alpha.alpha_token.A", Type: "alpha_token", Attrs: orig}); err != nil {
		t.Fatal(err)
	}

	mgr := plugin.NewManager()
	defer mgr.Close()
	res, err := refresh.Run(context.Background(), g, mgr, st)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if len(res.Refreshed) != 1 {
		t.Fatalf("refreshed = %v, want 1", res.Refreshed)
	}
	got, ok, _ := st.Get("alpha.alpha_token.A")
	if !ok {
		t.Fatal("A missing after refresh")
	}
	if got.Attrs["id"] != "alpha-0" || got.Attrs["value"] != "alpha::0" {
		t.Errorf("refresh changed converged state: %v", got.Attrs)
	}
}
