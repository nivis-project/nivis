// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package refresh_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/provider"
	"github.com/nivis-project/nivis/internal/refresh"
	"github.com/nivis-project/nivis/internal/state"
)

// versionedClient declares a schema version, records what it was asked to
// upgrade, and renames `tag` to `label` the way a real upgrader would.
type versionedClient struct {
	version      int64
	read         map[string]interface{}
	upgradeCalls int
	gotFrom      int64
}

func (c *versionedClient) GetSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{Version: c.version}, nil
}

func (c *versionedClient) Read(context.Context, provider.ReadRequest) (provider.ReadResult, error) {
	return provider.ReadResult{Attrs: c.read}, nil
}

func (c *versionedClient) UpgradeState(_ context.Context, req provider.UpgradeStateRequest) (provider.UpgradeStateResult, error) {
	c.upgradeCalls++
	c.gotFrom = req.FromVersion
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

func (c *versionedClient) Configure(context.Context, map[string]interface{}) error { return nil }
func (c *versionedClient) ListResourceTypes(context.Context) ([]string, error)     { return nil, nil }
func (c *versionedClient) Plan(context.Context, provider.PlanRequest) (provider.PlanResult, error) {
	return provider.PlanResult{}, nil
}
func (c *versionedClient) Apply(context.Context, provider.ApplyRequest) (provider.ApplyResult, error) {
	return provider.ApplyResult{}, nil
}
func (c *versionedClient) Destroy(context.Context, provider.DestroyRequest) (provider.DestroyResult, error) {
	return provider.DestroyResult{}, nil
}
func (c *versionedClient) GetDataSourceSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{}, nil
}
func (c *versionedClient) ReadDataSource(context.Context, provider.ReadDataSourceRequest) (provider.ReadDataSourceResult, error) {
	return provider.ReadDataSourceResult{}, nil
}

type versionedManager struct{ c *versionedClient }

func (m versionedManager) Client(string, string, map[string]interface{}) (provider.Client, error) {
	return m.c, nil
}
func (m versionedManager) Close() {}

// versionFixture builds a one-resource graph and a store seeded at a chosen
// schema version.
func versionFixture(t *testing.T, storedVersion int64, stored map[string]interface{}) (*ir.Graph, state.Store) {
	t.Helper()
	g, err := ir.IngestIR([]byte(`{
	  "schemaVersion":1,"providers":{"p":{"source":"stub","config":{}}},
	  "resources":[{"id":"p.t.r","provider":"p","type":"t","name":"r","config":{}}],
	  "edges":[],"nixConsumers":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	st, err2 := state.Open(t.TempDir() + "/state.json")
	if err2 != nil {
		t.Fatal(err2)
	}
	if err := st.Set(state.ResourceState{
		ID: "p.t.r", Type: "t", SchemaVersion: storedVersion, Attrs: stored,
	}); err != nil {
		t.Fatal(err)
	}
	return g, st
}

// TestRefreshUpgradesAnOlderVersion is the regression for bean nixform2-ff03:
// refresh is the command reached for when state looks stale, which is the same
// moment a provider has usually been upgraded.
func TestRefreshUpgradesAnOlderVersion(t *testing.T) {
	c := &versionedClient{version: 2, read: map[string]interface{}{"label": "kept"}}
	g, st := versionFixture(t, 1, map[string]interface{}{"tag": "kept"})

	if _, err := refresh.Run(context.Background(), g, versionedManager{c: c}, st, refresh.Options{}); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if c.upgradeCalls != 1 {
		t.Errorf("upgrade calls = %d, want 1", c.upgradeCalls)
	}
	if c.gotFrom != 1 {
		t.Errorf("upgraded from %d, want the version the STATE recorded (1)", c.gotFrom)
	}
}

func TestRefreshRefusesANewerVersion(t *testing.T) {
	c := &versionedClient{version: 1, read: map[string]interface{}{"label": "kept"}}
	g, st := versionFixture(t, 3, map[string]interface{}{"label": "kept"})

	_, err := refresh.Run(context.Background(), g, versionedManager{c: c}, st, refresh.Options{})
	if err == nil {
		t.Fatalf("expected refresh to refuse state written by a newer provider")
	}
	for _, want := range []string{"p.t.r", "newer", "version 3"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q: %v", want, err)
		}
	}
	// State must be untouched by a refusal.
	rs, _, _ := st.Get("p.t.r")
	if rs.SchemaVersion != 3 {
		t.Errorf("state was modified by a refused refresh: version is now %d", rs.SchemaVersion)
	}
}

func TestRefreshMakesNoUpgradeCallWhenVersionsMatch(t *testing.T) {
	c := &versionedClient{version: 2, read: map[string]interface{}{"label": "kept"}}
	g, st := versionFixture(t, 2, map[string]interface{}{"label": "kept"})

	if _, err := refresh.Run(context.Background(), g, versionedManager{c: c}, st, refresh.Options{}); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if c.upgradeCalls != 0 {
		t.Errorf("upgrade calls = %d, want 0 when the versions match", c.upgradeCalls)
	}
}

func TestRefreshMakesNoUpgradeCallWhenNoVersionRecorded(t *testing.T) {
	c := &versionedClient{version: 2, read: map[string]interface{}{"label": "kept"}}
	g, st := versionFixture(t, 0, map[string]interface{}{"label": "kept"})

	if _, err := refresh.Run(context.Background(), g, versionedManager{c: c}, st, refresh.Options{}); err != nil {
		t.Fatalf("a document with no recorded version must refresh as before: %v", err)
	}
	if c.upgradeCalls != 0 {
		t.Errorf("upgrade calls = %d, want 0: absent means current, not version zero", c.upgradeCalls)
	}
}

// The regression task 3.1 exists to prevent. A schema upgrade changes the shape
// by design; comparing the OLD shape against a fresh read reports the provider's
// own rename as infrastructure having changed underneath the user.
func TestAProviderRenameIsNotReportedAsDrift(t *testing.T) {
	c := &versionedClient{version: 2, read: map[string]interface{}{"label": "kept"}}
	g, st := versionFixture(t, 1, map[string]interface{}{"tag": "kept"})

	res, err := refresh.Run(context.Background(), g, versionedManager{c: c}, st, refresh.Options{})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if len(res.Drifted) != 0 {
		t.Errorf("drift reported for %v: the upgrade renamed tag to label and the provider "+
			"returned exactly that, so nothing changed underneath the user", res.Drifted)
	}
}

// The refreshed entry records the provider's CURRENT version explicitly, rather
// than defaulting to zero and relying on absent-means-current.
func TestRefreshRecordsTheCurrentVersion(t *testing.T) {
	c := &versionedClient{version: 4, read: map[string]interface{}{"label": "kept"}}
	g, st := versionFixture(t, 4, map[string]interface{}{"label": "kept"})

	if _, err := refresh.Run(context.Background(), g, versionedManager{c: c}, st, refresh.Options{}); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	rs, found, _ := st.Get("p.t.r")
	if !found || rs.SchemaVersion != 4 {
		t.Errorf("recorded version = %d, want 4", rs.SchemaVersion)
	}
}

// Import satisfies provider.Client. These stubs are not used for import tests.
func (c *versionedClient) Import(context.Context, provider.ImportRequest) (provider.ImportResult, error) {
	return provider.ImportResult{}, nil
}
