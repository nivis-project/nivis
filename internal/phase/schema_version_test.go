// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/phase"
	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/provider"
	"github.com/nivis-project/nivis/internal/state"
)

// versionClient reports a schema version and records what it was asked to
// upgrade, so a test can assert the CALL as well as the outcome. The rename it
// performs models a provider whose new schema calls an attribute something else.
type versionClient struct {
	version      int64
	upgradeCalls int
	gotFrom      int64
}

func (c *versionClient) Configure(context.Context, map[string]interface{}) error { return nil }
func (c *versionClient) ListResourceTypes(context.Context) ([]string, error)     { return nil, nil }
func (c *versionClient) GetSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{TypeName: "t", Version: c.version}, nil
}

func (c *versionClient) Plan(_ context.Context, req provider.PlanRequest) (provider.PlanResult, error) {
	// A no-op only when the prior actually carries the attribute the config
	// declares. A stub that reported no-op for ANY non-nil prior would pass
	// whether or not the state had been upgraded, which is exactly the trap:
	// the rename test must be able to fail.
	noop := req.Prior != nil && req.Prior["label"] != nil
	return provider.PlanResult{PlannedState: "planned", NoOp: noop}, nil
}

func (c *versionClient) Apply(_ context.Context, req provider.ApplyRequest) (provider.ApplyResult, error) {
	return provider.ApplyResult{Attrs: map[string]interface{}{"label": "v"}}, nil
}

func (c *versionClient) Read(_ context.Context, req provider.ReadRequest) (provider.ReadResult, error) {
	return provider.ReadResult{Attrs: req.Stored}, nil
}

func (c *versionClient) Destroy(context.Context, provider.DestroyRequest) (provider.DestroyResult, error) {
	return provider.DestroyResult{}, nil
}

func (c *versionClient) GetDataSourceSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{}, nil
}

func (c *versionClient) ReadDataSource(context.Context, provider.ReadDataSourceRequest) (provider.ReadDataSourceResult, error) {
	return provider.ReadDataSourceResult{}, nil
}

func (c *versionClient) UpgradeState(_ context.Context, req provider.UpgradeStateRequest) (provider.UpgradeStateResult, error) {
	c.upgradeCalls++
	c.gotFrom = req.FromVersion
	// The rename a real upgrader would perform: the old name carried across.
	out := map[string]interface{}{}
	for k, v := range req.Stored {
		if k == "tag" {
			out["label"] = v
			continue
		}
		out[k] = v
	}
	return provider.UpgradeStateResult{Attrs: out}, nil
}

type versionManager struct{ c *versionClient }

func (m versionManager) Client(string, string, map[string]interface{}) (provider.Client, error) {
	return m.c, nil
}
func (m versionManager) Close() {}

func versionIR(_ *ledger.Ledger) []byte {
	return []byte(`{
	  "schemaVersion":1,
	  "providers":{"p":{"source":"stub","config":{}}},
	  "resources":[{"id":"p.t.r","provider":"p","type":"t","name":"r","config":{"label":"v"}}],
	  "dataSources":[],"edges":[],"nixConsumers":[]
	}`)
}

// seed writes a stored resource at a chosen schema version, as an earlier run
// would have left it.
func seed(t *testing.T, version int64, attrs map[string]interface{}) (state.Store, string) {
	t.Helper()
	path := t.TempDir() + "/state.json"
	st, err := state.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Set(state.ResourceState{
		ID: "p.t.r", Type: "t", SchemaVersion: version, Attrs: attrs,
	}); err != nil {
		t.Fatal(err)
	}
	return st, path
}

func versionDriver(st state.Store, c *versionClient) *phase.Driver {
	return &phase.Driver{
		Eval:      &phase.StubEvaluator{IRForLedger: versionIR},
		Manager:   versionManager{c: c},
		Store:     st,
		Ledger:    ledger.New(),
		MaxPhases: 5,
	}
}

// TestOlderStoredVersionIsUpgraded is the regression for bean nixform2-67gg.
func TestOlderStoredVersionIsUpgraded(t *testing.T) {
	c := &versionClient{version: 2}
	st, _ := seed(t, 1, map[string]interface{}{"tag": "kept"})

	if _, err := versionDriver(st, c).Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}

	if c.upgradeCalls != 1 {
		t.Errorf("upgrade calls = %d, want 1", c.upgradeCalls)
	}
	if c.gotFrom != 1 {
		t.Errorf("upgraded from version %d, want the version the STATE was written at (1)", c.gotFrom)
	}
}

// A matching version costs nothing: the provider is not asked.
func TestMatchingVersionMakesNoUpgradeCall(t *testing.T) {
	c := &versionClient{version: 2}
	st, _ := seed(t, 2, map[string]interface{}{"label": "kept"})

	if _, err := versionDriver(st, c).Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if c.upgradeCalls != 0 {
		t.Errorf("upgrade calls = %d, want 0 when the versions match", c.upgradeCalls)
	}
}

// A document written before the field exists carries no version. It must behave
// exactly as it does today: current, no upgrade attempted, no failure. This is
// the regression the change could plausibly have caused.
func TestMissingVersionIsTreatedAsCurrent(t *testing.T) {
	c := &versionClient{version: 2}
	st, _ := seed(t, 0, map[string]interface{}{"label": "kept"})

	if _, err := versionDriver(st, c).Run(context.Background()); err != nil {
		t.Fatalf("a document with no recorded version must behave as today: %v", err)
	}
	if c.upgradeCalls != 0 {
		t.Errorf("upgrade calls = %d, want 0: absent means current, not version zero", c.upgradeCalls)
	}
}

// The other direction cannot be repaired, so it is refused.
func TestNewerStoredVersionIsRefused(t *testing.T) {
	c := &versionClient{version: 1}
	st, _ := seed(t, 3, map[string]interface{}{"label": "kept"})

	_, err := versionDriver(st, c).Run(context.Background())
	if err == nil {
		t.Fatalf("expected state written by a newer provider to be refused")
	}
	for _, want := range []string{"p.t.r", "newer", "version 3", "Upgrade the provider"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q: %v", want, err)
		}
	}
	if c.upgradeCalls != 0 {
		t.Errorf("a newer version must be refused, not upgraded; got %d calls", c.upgradeCalls)
	}
}

// The headline case: a provider renames an attribute between versions, and the
// plan after the upgrade reports NO CHANGE. Without the upgrade the old name is
// dropped and the new one is null, which is a change out of nothing.
func TestRenamedAttributeDoesNotBecomeAChange(t *testing.T) {
	c := &versionClient{version: 2}
	st, _ := seed(t, 1, map[string]interface{}{"tag": "kept"})

	res, err := versionDriver(st, c).Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	var op plan.Op = -1
	for _, ph := range res.Phases {
		for _, n := range ph {
			if n.ID == "p.t.r" {
				op = n.Op
			}
		}
	}
	if op != plan.OpNoop {
		t.Errorf("resource resolved as %v, want a no-op: the upgrade carried `tag` across to `label`, "+
			"so there is nothing to change", op)
	}
}

// Import satisfies provider.Client. These stubs are not used for import tests.
func (c *versionClient) Import(context.Context, provider.ImportRequest) (provider.ImportResult, error) {
	return provider.ImportResult{}, nil
}
