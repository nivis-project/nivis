// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/wearetechnative/terrae-nivis/internal/ir"
	"github.com/wearetechnative/terrae-nivis/internal/ledger"
	"github.com/wearetechnative/terrae-nivis/internal/provider"
	"github.com/wearetechnative/terrae-nivis/internal/state"
)

// recordClient records the create/update/replace mechanics: which prior state
// Plan/Apply received and whether Destroy was called. RequiresReplace is
// configurable; Apply echoes a deterministic new attr so we can see state move.
type recordClient struct {
	requiresReplace bool
	noop            bool

	planPrior    map[string]interface{}
	applyCalls   int
	applyPrior   map[string]interface{}
	destroyCalls int
	destroyAttrs map[string]interface{}
}

func (c *recordClient) Configure(context.Context, map[string]interface{}) error { return nil }
func (c *recordClient) ListResourceTypes(context.Context) ([]string, error)     { return nil, nil }
func (c *recordClient) GetSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{TypeName: "t"}, nil
}
func (c *recordClient) Plan(_ context.Context, req provider.PlanRequest) (provider.PlanResult, error) {
	c.planPrior = req.Prior
	return provider.PlanResult{
		PlannedState:    "planned",
		RequiresReplace: req.Prior != nil && c.requiresReplace,
		NoOp:            req.Prior != nil && c.noop,
	}, nil
}
func (c *recordClient) Apply(_ context.Context, req provider.ApplyRequest) (provider.ApplyResult, error) {
	c.applyCalls++
	c.applyPrior = req.Prior
	return provider.ApplyResult{Attrs: map[string]interface{}{"id": "new"}}, nil
}
func (c *recordClient) Read(context.Context, provider.ReadRequest) (provider.ReadResult, error) {
	return provider.ReadResult{}, nil
}
func (c *recordClient) Destroy(_ context.Context, req provider.DestroyRequest) (provider.DestroyResult, error) {
	c.destroyCalls++
	c.destroyAttrs = req.Stored
	return provider.DestroyResult{}, nil
}

type recordManager struct{ c *recordClient }

func (m recordManager) Client(string, string, map[string]interface{}) (provider.Client, error) {
	return m.c, nil
}

func newReplaceDriver(t *testing.T, c *recordClient) *Driver {
	t.Helper()
	st, err := state.Open(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	return &Driver{Manager: recordManager{c: c}, Store: st, Ledger: ledger.New()}
}

func replaceGraphNode(meta *ir.Meta) (*ir.Graph, *ir.ResourceNode) {
	node := &ir.ResourceNode{Resource: ir.Resource{
		ID: "aws.aws_s3_bucket.demo", Provider: "aws", Type: "aws_s3_bucket", Name: "demo", Meta: meta,
	}}
	g := &ir.Graph{
		Providers: map[string]ir.ProviderConfig{"aws": {Source: "aws"}},
		Nodes:     map[string]*ir.ResourceNode{node.Resource.ID: node},
		Order:     []string{node.Resource.ID},
	}
	return g, node
}

// No prior state: a plain create. No destroy; nil prior to plan/apply.
func TestApplyOneCreate(t *testing.T) {
	c := &recordClient{}
	d := newReplaceDriver(t, c)
	g, node := replaceGraphNode(nil)

	if _, err := d.applyOne(context.Background(), g, node, map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}
	if c.destroyCalls != 0 {
		t.Errorf("create must not destroy; destroyCalls=%d", c.destroyCalls)
	}
	if c.planPrior != nil || c.applyPrior != nil {
		t.Errorf("create must send nil prior; plan=%v apply=%v", c.planPrior, c.applyPrior)
	}
}

// Prior in state + no replace: update in place. Prior flows to plan AND apply; no destroy.
func TestApplyOneUpdateInPlace(t *testing.T) {
	c := &recordClient{requiresReplace: false}
	d := newReplaceDriver(t, c)
	g, node := replaceGraphNode(nil)
	if err := d.Store.Set(state.ResourceState{ID: node.Resource.ID, Type: node.Resource.Type, Attrs: map[string]interface{}{"id": "old"}}); err != nil {
		t.Fatal(err)
	}

	if _, err := d.applyOne(context.Background(), g, node, map[string]interface{}{}); err != nil {
		t.Fatal(err)
	}
	if c.destroyCalls != 0 {
		t.Errorf("in-place update must not destroy; destroyCalls=%d", c.destroyCalls)
	}
	if c.applyPrior["id"] != "old" {
		t.Errorf("update must apply against prior state; applyPrior=%v", c.applyPrior)
	}
}

// Prior in state + RequiresReplace: destroy the prior, then create. Apply runs as
// a create (nil prior); destroy got the old attrs; state holds the new attrs.
func TestApplyOneReplaceDestroysThenCreates(t *testing.T) {
	c := &recordClient{requiresReplace: true}
	d := newReplaceDriver(t, c)
	g, node := replaceGraphNode(nil)
	if err := d.Store.Set(state.ResourceState{ID: node.Resource.ID, Type: node.Resource.Type, Attrs: map[string]interface{}{"id": "old"}}); err != nil {
		t.Fatal(err)
	}

	attrs, err := d.applyOne(context.Background(), g, node, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if c.destroyCalls != 1 {
		t.Errorf("replace must destroy the prior exactly once; destroyCalls=%d", c.destroyCalls)
	}
	if c.destroyAttrs["id"] != "old" {
		t.Errorf("replace must destroy the OLD resource; destroyAttrs=%v", c.destroyAttrs)
	}
	if c.applyPrior != nil {
		t.Errorf("the create half of a replace must send nil prior; applyPrior=%v", c.applyPrior)
	}
	if attrs["id"] != "new" {
		t.Errorf("state should reflect the new resource; got %v", attrs)
	}
}

// Prior in state + the provider reports a no-op: applyOne touches neither Apply
// nor Destroy, and surfaces the prior attributes as the outputs (so dependents
// resolve). This is the beans-l2q2 fix: a re-apply of an unchanged resource.
func TestApplyOneNoopSkipsApply(t *testing.T) {
	c := &recordClient{noop: true}
	d := newReplaceDriver(t, c)
	g, node := replaceGraphNode(nil)
	if err := d.Store.Set(state.ResourceState{ID: node.Resource.ID, Type: node.Resource.Type, Attrs: map[string]interface{}{"id": "old"}}); err != nil {
		t.Fatal(err)
	}

	outs, err := d.applyOne(context.Background(), g, node, map[string]interface{}{})
	if err != nil {
		t.Fatal(err)
	}
	if c.applyCalls != 0 {
		t.Errorf("no-op must not call Apply; applyCalls=%d", c.applyCalls)
	}
	if c.destroyCalls != 0 {
		t.Errorf("no-op must not call Destroy; destroyCalls=%d", c.destroyCalls)
	}
	if outs["id"] != "old" {
		t.Errorf("no-op must surface the prior attrs as outputs; got %v", outs)
	}
}

// preventDestroy refuses a replace instead of destroying the protected resource.
func TestApplyOneReplaceRefusedByPreventDestroy(t *testing.T) {
	c := &recordClient{requiresReplace: true}
	d := newReplaceDriver(t, c)
	g, node := replaceGraphNode(&ir.Meta{Lifecycle: &ir.Lifecycle{PreventDestroy: true}})
	if err := d.Store.Set(state.ResourceState{ID: node.Resource.ID, Type: node.Resource.Type, Attrs: map[string]interface{}{"id": "old"}}); err != nil {
		t.Fatal(err)
	}

	_, err := d.applyOne(context.Background(), g, node, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected an error refusing the replace")
	}
	if c.destroyCalls != 0 {
		t.Errorf("preventDestroy must not destroy; destroyCalls=%d", c.destroyCalls)
	}
}
