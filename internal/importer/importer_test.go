// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package importer_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/importer"
	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/provider"
	"github.com/nivis-project/nivis/internal/state"
)

// importClient models a provider's import: a stub carrying the identity, then a
// read that populates it. applyCalls is recorded so a test can assert that
// importing creates NOTHING, rather than only that nothing appears changed.
type importClient struct {
	version   int64
	returns   int  // how many resources one import yields
	readEmpty bool // the object does not exist
	readAttrs map[string]interface{}
	// schemaAttrs are the attribute names the schema declares, so the read can
	// return the ones the provider does not populate as nil.
	schemaAttrs []string
	applyCalls  int
}

func (c *importClient) Import(_ context.Context, req provider.ImportRequest) (provider.ImportResult, error) {
	n := c.returns
	if n == 0 {
		n = 1
	}
	var out []provider.ImportedResource
	for range n {
		out = append(out, provider.ImportedResource{
			TypeName: req.TypeName,
			Attrs:    map[string]interface{}{"id": req.ImportID},
		})
	}
	return provider.ImportResult{Resources: out}, nil
}

func (c *importClient) Read(_ context.Context, req provider.ReadRequest) (provider.ReadResult, error) {
	if c.readEmpty {
		return provider.ReadResult{}, nil
	}
	attrs := map[string]interface{}{}
	for k, v := range c.readAttrs {
		attrs[k] = v
	}
	attrs["id"] = req.Stored["id"]
	// A real read goes through DecodeState, which emits EVERY attribute in the
	// schema, so one the provider does not return comes back as nil rather than
	// being absent. That nil is what would overwrite a seeded value applied
	// before the read, so a stub omitting it cannot test the ordering.
	for _, name := range c.schemaAttrs {
		if _, ok := attrs[name]; !ok {
			attrs[name] = nil
		}
	}
	return provider.ReadResult{Attrs: attrs}, nil
}

func (c *importClient) Apply(context.Context, provider.ApplyRequest) (provider.ApplyResult, error) {
	c.applyCalls++
	return provider.ApplyResult{}, nil
}

func (c *importClient) GetSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{TypeName: "t", Version: c.version}, nil
}
func (c *importClient) Configure(context.Context, map[string]interface{}) error { return nil }
func (c *importClient) ListResourceTypes(context.Context) ([]string, error)     { return nil, nil }
func (c *importClient) Plan(context.Context, provider.PlanRequest) (provider.PlanResult, error) {
	return provider.PlanResult{}, nil
}
func (c *importClient) Destroy(context.Context, provider.DestroyRequest) (provider.DestroyResult, error) {
	return provider.DestroyResult{}, nil
}
func (c *importClient) GetDataSourceSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{}, nil
}
func (c *importClient) ReadDataSource(context.Context, provider.ReadDataSourceRequest) (provider.ReadDataSourceResult, error) {
	return provider.ReadDataSourceResult{}, nil
}
func (c *importClient) UpgradeState(_ context.Context, req provider.UpgradeStateRequest) (provider.UpgradeStateResult, error) {
	return provider.UpgradeStateResult{Attrs: req.Stored}, nil
}

type importManager struct{ c *importClient }

func (m importManager) Client(string, string, map[string]interface{}) (provider.Client, error) {
	return m.c, nil
}

func fixture(t *testing.T) (*ir.Graph, state.Store) {
	t.Helper()
	g, err := ir.IngestIR([]byte(`{
	  "schemaVersion":1,"providers":{"p":{"source":"stub","config":{}}},
	  "resources":[{"id":"p.t.r","provider":"p","type":"t","name":"r","config":{}}],
	  "edges":[],"nixConsumers":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	st, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatal(err)
	}
	return g, st
}

func manifest(id string, seed map[string]interface{}) importer.Manifest {
	return importer.Manifest{Imports: map[string]importer.Entry{
		"p.t.r": {ID: id, Seed: seed},
	}}
}

func run(t *testing.T, c *importClient, g *ir.Graph, st state.Store, m importer.Manifest) *importer.Summary {
	t.Helper()
	sum, err := importer.Run(context.Background(), g, importManager{c: c}, st, m)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	return sum
}

func TestImportAdoptsAResource(t *testing.T) {
	c := &importClient{version: 2, readAttrs: map[string]interface{}{"label": "live"}}
	g, st := fixture(t)

	sum := run(t, c, g, st, manifest("real-id", nil))

	if sum.Imported != 1 || sum.Refused != 0 {
		t.Fatalf("imported=%d refused=%d, want 1 and 0: %v", sum.Imported, sum.Refused, sum.Results[0].Err)
	}
	rs, found, _ := st.Get("p.t.r")
	if !found {
		t.Fatalf("expected the resource in state")
	}
	if rs.Attrs["label"] != "live" || rs.Attrs["id"] != "real-id" {
		t.Errorf("state = %#v, want the provider's view", rs.Attrs)
	}
	if rs.SchemaVersion != 2 {
		t.Errorf("recorded schema version = %d, want 2", rs.SchemaVersion)
	}
}

// Importing must create nothing. Asserted on the provider having received no
// apply, not merely on the absence of visible change.
func TestImportCreatesNothing(t *testing.T) {
	c := &importClient{readAttrs: map[string]interface{}{"label": "live"}}
	g, st := fixture(t)

	run(t, c, g, st, manifest("real-id", nil))

	if c.applyCalls != 0 {
		t.Errorf("provider received %d apply calls; importing must create nothing", c.applyCalls)
	}
}

// A seeded value is taken on trust and must survive the read, which returns the
// provider's full view and by definition excludes it.
func TestSeededValueSurvivesTheRead(t *testing.T) {
	c := &importClient{
		readAttrs:   map[string]interface{}{"label": "live"},
		schemaAttrs: []string{"label", "id", "secret"},
	}
	g, st := fixture(t)

	sum := run(t, c, g, st, manifest("real-id", map[string]interface{}{"secret": "s3cret"}))

	rs, _, _ := st.Get("p.t.r")
	if rs.Attrs["secret"] != "s3cret" {
		t.Errorf("secret = %v, want the seeded value", rs.Attrs["secret"])
	}
	if rs.Attrs["label"] != "live" {
		t.Errorf("seeding overwrote what the provider returned: %#v", rs.Attrs)
	}
	if len(sum.Results[0].Seeded) != 1 || sum.Results[0].Seeded[0] != "secret" {
		t.Errorf("Seeded = %v, want [secret] so a reader can tell trust from observation", sum.Results[0].Seeded)
	}
}

func TestRefusesAnUndeclaredID(t *testing.T) {
	c := &importClient{readAttrs: map[string]interface{}{"label": "live"}}
	g, st := fixture(t)
	m := importer.Manifest{Imports: map[string]importer.Entry{"p.t.nope": {ID: "x"}}}

	sum := run(t, c, g, st, m)

	if sum.Refused != 1 {
		t.Fatalf("want the entry refused")
	}
	if !strings.Contains(sum.Results[0].Err.Error(), "not declared") {
		t.Errorf("error does not say it must be declared: %v", sum.Results[0].Err)
	}
	if _, found, _ := st.Get("p.t.nope"); found {
		t.Errorf("state was written for a refused entry")
	}
}

func TestRefusesAMultiResourceResponseAndNamesWhatCameBack(t *testing.T) {
	c := &importClient{returns: 3, readAttrs: map[string]interface{}{"label": "live"}}
	g, st := fixture(t)

	sum := run(t, c, g, st, manifest("real-id", nil))

	if sum.Refused != 1 {
		t.Fatalf("want the entry refused")
	}
	msg := sum.Results[0].Err.Error()
	for _, want := range []string{"3 resources", "state push", "t"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not mention %q: %v", want, msg)
		}
	}
	if _, found, _ := st.Get("p.t.r"); found {
		t.Errorf("state was written for a refused entry")
	}
}

// An empty read means the id identified nothing. It must NOT be read the way
// nivis reads an empty refresh, where it means an out-of-band deletion and
// plans a create: here that would duplicate something that may well exist.
func TestRefusesWhenTheObjectDoesNotExist(t *testing.T) {
	c := &importClient{readEmpty: true}
	g, st := fixture(t)

	sum := run(t, c, g, st, manifest("wrong-id", nil))

	if sum.Refused != 1 {
		t.Fatalf("want the entry refused")
	}
	msg := sum.Results[0].Err.Error()
	for _, want := range []string{"no object exists", "wrong-id"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error does not mention %q: %v", want, msg)
		}
	}
	if _, found, _ := st.Get("p.t.r"); found {
		t.Errorf("state was written for an object that does not exist")
	}
}

func TestRefusesAResourceAlreadyInState(t *testing.T) {
	c := &importClient{readAttrs: map[string]interface{}{"label": "live"}}
	g, st := fixture(t)
	if err := st.Set(state.ResourceState{ID: "p.t.r", Type: "t", Attrs: map[string]interface{}{"label": "existing"}}); err != nil {
		t.Fatal(err)
	}

	sum := run(t, c, g, st, manifest("real-id", nil))

	if sum.Refused != 1 {
		t.Fatalf("want the entry refused")
	}
	rs, _, _ := st.Get("p.t.r")
	if rs.Attrs["label"] != "existing" {
		t.Errorf("the existing entry was overwritten: %#v", rs.Attrs)
	}
}

// A run over many resources that fails on some must leave the successes in
// place and be resumable by fixing the failures.
func TestPartialFailureLeavesTheSuccessesImported(t *testing.T) {
	c := &importClient{readAttrs: map[string]interface{}{"label": "live"}}
	g, st := fixture(t)
	m := importer.Manifest{Imports: map[string]importer.Entry{
		"p.t.r":    {ID: "real-id"},
		"p.t.nope": {ID: "x"}, // undeclared
	}}

	sum := run(t, c, g, st, m)

	if sum.Imported != 1 || sum.Refused != 1 {
		t.Fatalf("imported=%d refused=%d, want 1 and 1", sum.Imported, sum.Refused)
	}
	if _, found, _ := st.Get("p.t.r"); !found {
		t.Errorf("the successful entry is not in state")
	}
	if _, found, _ := st.Get("p.t.nope"); found {
		t.Errorf("the refused entry left state behind")
	}
}

func TestManifestIsValidatedBeforeAnyProviderIsContacted(t *testing.T) {
	if _, err := importer.ParseManifest([]byte(`not json`)); err == nil {
		t.Errorf("expected malformed JSON to be refused")
	}
	if _, err := importer.ParseManifest([]byte(`{"imports":{}}`)); err == nil {
		t.Errorf("expected an empty manifest to be refused")
	}
	_, err := importer.ParseManifest([]byte(`{"imports":{"p.t.r":{}}}`))
	if err == nil || !strings.Contains(err.Error(), "no \"id\"") {
		t.Errorf("expected an entry without an id to be refused, got %v", err)
	}
}
