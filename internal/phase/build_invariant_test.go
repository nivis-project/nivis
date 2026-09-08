// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase

// THE INVARIANT: no config handed to a provider ever carries a `__build` leaf —
// on any path.
//
// This is the test the defect would have failed. `realiseBuilds` used to
// substitute leaves and was called from exactly one of five paths that hand a
// config to a provider, so a plan of a resource, and a datasource read during
// plan, apply or outputs resolution, all delivered a raw leaf to the provider's
// config encoder ("expected string, got map[string]interface {}", nixform2-hytv).
//
// Substitution now happens in the resolve pass, so this asserts the property
// itself rather than the four fixed call sites: it fails if substitution is
// removed from graph.ResolveTFTF, and it keeps failing for any future path.

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/provider"
	"github.com/nivis-project/nivis/internal/state"
)

// buildLeafIR is one resource and one datasource, each carrying a `__build` leaf.
const buildLeafIR = `{
  "schemaVersion":1,
  "providers":{"alpha":{"source":"provider-alpha","config":{}}},
  "resources":[
    {"id":"alpha.alpha_token.probe","provider":"alpha","type":"alpha_token","name":"probe",
     "config":{"label":{"__build":{"path":"/nix/store/aaa-img/x.vhd","drv":"/nix/store/ddd-img.drv"}}}}
  ],
  "dataSources":[
    {"id":"data.alpha.alpha_lookup.probe","provider":"alpha","type":"alpha_lookup","name":"probe",
     "config":{"query":{"__build":{"path":"/nix/store/bbb-pkg","drv":"/nix/store/eee-pkg.drv"}}}}
  ],
  "edges":[],
  "nixConsumers":[]
}`

// fixedEval returns one IR, whatever the ledger says.
type fixedEval struct{ ir string }

func (f fixedEval) Eval(context.Context, *ledger.Ledger) ([]byte, error) { return []byte(f.ir), nil }

// cfgRecorder is a provider client that records every config it is handed, on
// every RPC that takes one.
type cfgRecorder struct {
	configs map[string]map[string]interface{} // rpc -> config
}

func newCfgRecorder() *cfgRecorder {
	return &cfgRecorder{configs: map[string]map[string]interface{}{}}
}

func (c *cfgRecorder) record(rpc string, cfg map[string]interface{}) {
	c.configs[rpc] = cfg
}

func (c *cfgRecorder) Configure(context.Context, map[string]interface{}) error { return nil }
func (c *cfgRecorder) ListResourceTypes(context.Context) ([]string, error)     { return nil, nil }
func (c *cfgRecorder) GetSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{TypeName: "alpha_token"}, nil
}
func (c *cfgRecorder) GetDataSourceSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{TypeName: "alpha_lookup"}, nil
}
func (c *cfgRecorder) Plan(_ context.Context, req provider.PlanRequest) (provider.PlanResult, error) {
	c.record("plan", req.ResolvedCfg)
	return provider.PlanResult{PlannedState: "planned", NoOp: req.Prior != nil}, nil
}
func (c *cfgRecorder) Apply(_ context.Context, req provider.ApplyRequest) (provider.ApplyResult, error) {
	c.record("apply", req.ResolvedCfg)
	return provider.ApplyResult{Attrs: map[string]interface{}{"id": "x", "value": "v"}}, nil
}
func (c *cfgRecorder) ReadDataSource(_ context.Context, req provider.ReadDataSourceRequest) (provider.ReadDataSourceResult, error) {
	c.record("readDataSource", req.ResolvedCfg)
	return provider.ReadDataSourceResult{Attrs: map[string]interface{}{"result": "found"}}, nil
}
func (c *cfgRecorder) Read(_ context.Context, req provider.ReadRequest) (provider.ReadResult, error) {
	return provider.ReadResult{Attrs: req.Stored}, nil
}
func (c *cfgRecorder) Destroy(context.Context, provider.DestroyRequest) (provider.DestroyResult, error) {
	return provider.DestroyResult{}, nil
}

type recorderManager struct{ c *cfgRecorder }

func (m recorderManager) Client(string, string, map[string]interface{}) (provider.Client, error) {
	return m.c, nil
}

// findBuildLeaf returns the path to a surviving `__build` leaf, or "".
func findBuildLeaf(v interface{}, where string) string {
	switch t := v.(type) {
	case map[string]interface{}:
		if _, ok := t["__build"]; ok {
			return where
		}
		for k, child := range t {
			if hit := findBuildLeaf(child, where+"."+k); hit != "" {
				return hit
			}
		}
	case []interface{}:
		for i, child := range t {
			if hit := findBuildLeaf(child, fmt.Sprintf("%s[%d]", where, i)); hit != "" {
				return hit
			}
		}
	}
	return ""
}

// assertNoLeaves fails naming the RPC and the attribute path of any survivor.
func assertNoLeaves(t *testing.T, c *cfgRecorder, wantRPCs ...string) {
	t.Helper()
	for _, rpc := range wantRPCs {
		cfg, ok := c.configs[rpc]
		if !ok {
			t.Fatalf("%s was never called, so the invariant is untested on that path", rpc)
		}
		if hit := findBuildLeaf(cfg, rpc); hit != "" {
			t.Errorf("a __build leaf reached the provider on %s at %s: %#v", rpc, hit, cfg)
		}
	}
}

// invariantDriver wires a driver over the leaf-carrying IR and the recorder.
func invariantDriver(t *testing.T, c *cfgRecorder) *Driver {
	t.Helper()
	st, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return &Driver{
		Eval:    fixedEval{ir: buildLeafIR},
		Manager: recorderManager{c: c},
		Store:   st,
		Ledger:  ledger.New(),
		// The paths in the IR are fictional, so realising them is not the point
		// here; substitution is. --no-build is exactly that stance.
		NoBuild:   true,
		NoRefresh: true,
	}
}

// PLAN: both the resource plan and the datasource read.
func TestInvariantPlanReport(t *testing.T) {
	c := newCfgRecorder()
	d := invariantDriver(t, c)
	// A resource plan reaches the provider only for a resource already in state.
	if err := d.Store.Set(state.ResourceState{
		ID: "alpha.alpha_token.probe", Type: "alpha_token",
		Attrs: map[string]interface{}{"id": "x", "value": "v"},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := d.PlanReport(context.Background()); err != nil {
		t.Fatalf("PlanReport: %v", err)
	}
	assertNoLeaves(t, c, "plan", "readDataSource")
}

// APPLY: the resource apply and the datasource read inside the phase loop.
func TestInvariantApplyLoop(t *testing.T) {
	c := newCfgRecorder()
	d := invariantDriver(t, c)

	if _, err := d.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertNoLeaves(t, c, "plan", "apply", "readDataSource")
}

// OUTPUTS: the datasource read during outputs resolution.
func TestInvariantResolveOutputs(t *testing.T) {
	c := newCfgRecorder()
	d := invariantDriver(t, c)

	if _, err := d.ResolveOutputs(context.Background()); err != nil {
		t.Fatalf("ResolveOutputs: %v", err)
	}
	assertNoLeaves(t, c, "readDataSource")
}

// The apply path still learns what to build, even though the leaves are gone
// from the config by the time it runs: the resolve pass reports them.
func TestApplyStillLearnsWhatToBuild(t *testing.T) {
	c := newCfgRecorder()
	d := invariantDriver(t, c)
	sr := &stubRealiser{dontCreate: true}
	d.Realiser = sr
	d.NoBuild = false // we want the realise attempt, to see what it was asked for

	// The post-check fails (the fictional path cannot appear), which is fine: the
	// assertion is about WHAT was requested.
	_, _ = d.Run(context.Background())

	if len(sr.realised) == 0 {
		t.Fatal("the apply path realised nothing: the builds report did not reach it")
	}
	got := sr.realised[0]
	if got.Path != "/nix/store/aaa-img/x.vhd" || got.Drv != "/nix/store/ddd-img.drv" {
		t.Errorf("realised %+v, want the resource's build output with its derivation", got)
	}
}
