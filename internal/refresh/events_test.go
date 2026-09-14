// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package refresh_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/progress"
	"github.com/nivis-project/nivis/internal/provider"
	"github.com/nivis-project/nivis/internal/refresh"
	"github.com/nivis-project/nivis/internal/state"
)

type rec struct{ events []progress.Event }

func (r *rec) Emit(e progress.Event) { r.events = append(r.events, e) }

// refreshFixture: one resource in state, seeded with the given attrs. The fake
// provider's ReadResource echoes current state, so seeding attrs that differ
// from what it will return is how drift is simulated.
func refreshFixture(t *testing.T, attrs map[string]interface{}) (*ir.Graph, *plugin.Manager, state.Store) {
	t.Helper()
	bin := buildAlpha(t)
	g, err := ir.IngestIR([]byte(`{
	  "schemaVersion":1,"providers":{"alpha":{"source":"` + bin + `","config":{}}},
	  "resources":[{"id":"alpha.alpha_token.A","provider":"alpha","type":"alpha_token","name":"A","config":{}}],
	  "edges":[],"nixConsumers":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	st, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err := st.Set(state.ResourceState{ID: "alpha.alpha_token.A", Type: "alpha_token", Attrs: attrs}); err != nil {
		t.Fatal(err)
	}
	return g, plugin.NewManager(), st
}

// A refresh reports each resource as it is read. Before this it produced nothing
// at all — and a refresh is one provider round trip per resource in state, which
// on a large stack is the whole command.
func TestRefreshReportsEachNode(t *testing.T) {
	g, mgr, st := refreshFixture(t, map[string]interface{}{"id": "alpha-0", "value": "alpha::0"})
	defer mgr.Close()

	r := &rec{}
	res, err := refresh.Run(context.Background(), g, mgr, st, refresh.Options{Observer: r})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	var starts, dones int
	for _, e := range r.events {
		switch e.Kind {
		case progress.NodeStart:
			starts++
		case progress.NodeDone:
			dones++
		}
	}
	if starts != len(res.Refreshed) || dones != len(res.Refreshed) {
		t.Errorf("starts=%d dones=%d, want %d each", starts, dones, len(res.Refreshed))
	}
}

// Drift is the part of a refresh worth seeing. The count refreshed says what
// HAPPENED; this says what CHANGED, so a renderer can name the few that matter
// instead of all of them.
//
// This uses a stub client rather than the fake provider binary: the fake's
// ReadResource echoes CurrentState back verbatim, so no amount of seeding makes
// it report a different value. A stub is also hermetic and needs no build.
func TestRefreshReportsDrift(t *testing.T) {
	g, err := ir.IngestIR([]byte(`{
	  "schemaVersion":1,"providers":{"alpha":{"source":"unused","config":{}}},
	  "resources":[{"id":"alpha.alpha_token.A","provider":"alpha","type":"alpha_token","name":"A","config":{}}],
	  "edges":[],"nixConsumers":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	st, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err := st.Set(state.ResourceState{
		ID: "alpha.alpha_token.A", Type: "alpha_token",
		Attrs: map[string]interface{}{"id": "alpha-0", "value": "as-stored"},
	}); err != nil {
		t.Fatal(err)
	}

	// The provider reports a different value than what is stored: real drift.
	mgr := stubManager{client: &stubClient{read: map[string]interface{}{
		"id": "alpha-0", "value": "changed-underneath",
	}}}

	r := &rec{}
	res, err := refresh.Run(context.Background(), g, mgr, st, refresh.Options{Observer: r})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if len(res.Drifted) != 1 || res.Drifted[0] != "alpha.alpha_token.A" {
		t.Fatalf("Drifted = %v, want the one changed resource", res.Drifted)
	}
	var sawDrift bool
	for _, e := range r.events {
		if e.Kind == progress.NodeDone && e.Drifted {
			sawDrift = true
		}
	}
	if !sawDrift {
		t.Error("no event reported drift, so a renderer could not single it out")
	}
}

// The schema round trip must not be mistaken for drift. A provider read decodes
// at the FULL schema, so it returns attributes the stored state never had, as
// nil. Reporting that as drift would mark every resource in a stack as drifted
// and make the whole "3 of 40 drifted" summary a lie.
func TestRefreshSchemaPaddingIsNotDrift(t *testing.T) {
	g, err := ir.IngestIR([]byte(`{
	  "schemaVersion":1,"providers":{"alpha":{"source":"unused","config":{}}},
	  "resources":[{"id":"alpha.alpha_token.A","provider":"alpha","type":"alpha_token","name":"A","config":{}}],
	  "edges":[],"nixConsumers":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	st, _ := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err := st.Set(state.ResourceState{
		ID: "alpha.alpha_token.A", Type: "alpha_token",
		Attrs: map[string]interface{}{"id": "alpha-0", "value": "alpha::0"},
	}); err != nil {
		t.Fatal(err)
	}

	// Exactly what the real fake returns for a converged resource: the same
	// values plus the schema's other attribute, as nil.
	mgr := stubManager{client: &stubClient{read: map[string]interface{}{
		"id": "alpha-0", "value": "alpha::0", "label": nil,
	}}}

	res, err := refresh.Run(context.Background(), g, mgr, st, refresh.Options{})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if len(res.Drifted) != 0 {
		t.Errorf("Drifted = %v; a schema-padded read of an unchanged resource is not drift", res.Drifted)
	}
}

// A converged store reports no drift: the flag must mean something.
func TestRefreshConvergedReportsNoDrift(t *testing.T) {
	g, mgr, st := refreshFixture(t, map[string]interface{}{"id": "alpha-0", "value": "alpha::0"})
	defer mgr.Close()

	r := &rec{}
	res, err := refresh.Run(context.Background(), g, mgr, st, refresh.Options{Observer: r})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if len(res.Drifted) != 0 {
		t.Errorf("Drifted = %v, want none on a converged store", res.Drifted)
	}
	for _, e := range r.events {
		if e.Kind == progress.NodeDone && e.Drifted {
			t.Errorf("%s reported drift on a converged store", e.ID)
		}
	}
}

// A nil observer discards rather than panicking.
func TestRefreshWithoutObserver(t *testing.T) {
	g, mgr, st := refreshFixture(t, map[string]interface{}{"id": "alpha-0", "value": "alpha::0"})
	defer mgr.Close()
	if _, err := refresh.Run(context.Background(), g, mgr, st, refresh.Options{}); err != nil {
		t.Fatalf("refresh: %v", err)
	}
}

// stubManager hands out one stubClient, so a refresh can be driven without a
// provider binary.
type stubManager struct{ client *stubClient }

func (m stubManager) Client(string, string, map[string]interface{}) (provider.Client, error) {
	return m.client, nil
}

// stubClient implements provider.Client far enough for a refresh: a schema and
// a Read. Everything else is unreachable from this path and says so rather than
// returning a plausible zero value.
type stubClient struct{ read map[string]interface{} }

func (c *stubClient) Read(context.Context, provider.ReadRequest) (provider.ReadResult, error) {
	return provider.ReadResult{Attrs: c.read}, nil
}

func (c *stubClient) GetSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{}, nil
}

func (c *stubClient) Configure(context.Context, map[string]interface{}) error { return nil }

func (c *stubClient) ListResourceTypes(context.Context) ([]string, error) {
	return nil, errNotUsed
}

func (c *stubClient) GetDataSourceSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{}, errNotUsed
}

func (c *stubClient) Plan(context.Context, provider.PlanRequest) (provider.PlanResult, error) {
	return provider.PlanResult{}, errNotUsed
}

func (c *stubClient) Apply(context.Context, provider.ApplyRequest) (provider.ApplyResult, error) {
	return provider.ApplyResult{}, errNotUsed
}

func (c *stubClient) ReadDataSource(context.Context, provider.ReadDataSourceRequest) (provider.ReadDataSourceResult, error) {
	return provider.ReadDataSourceResult{}, errNotUsed
}

func (c *stubClient) Destroy(context.Context, provider.DestroyRequest) (provider.DestroyResult, error) {
	return provider.DestroyResult{}, errNotUsed
}

var errNotUsed = errors.New("stubClient: this method is not reachable from a refresh")
