// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"testing"

	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/provider"
)

// stubClient is an in-memory provider.Client for unit-testing the plan engine's
// create/update/replace decision without a real provider. It records the prior
// state it was sent and returns a configurable RequiresReplace.
type stubClient struct {
	requiresReplace bool
	noop            bool
	gotPrior        map[string]interface{}
}

func (s *stubClient) Configure(context.Context, map[string]interface{}) error { return nil }
func (s *stubClient) ListResourceTypes(context.Context) ([]string, error)     { return nil, nil }
func (s *stubClient) GetSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{}, nil
}
func (s *stubClient) Plan(_ context.Context, req provider.PlanRequest) (provider.PlanResult, error) {
	s.gotPrior = req.Prior
	return provider.PlanResult{PlannedState: "planned", RequiresReplace: s.requiresReplace, NoOp: s.noop}, nil
}
func (s *stubClient) Apply(context.Context, provider.ApplyRequest) (provider.ApplyResult, error) {
	return provider.ApplyResult{}, nil
}
func (s *stubClient) Read(context.Context, provider.ReadRequest) (provider.ReadResult, error) {
	return provider.ReadResult{}, nil
}
func (s *stubClient) Destroy(context.Context, provider.DestroyRequest) (provider.DestroyResult, error) {
	return provider.DestroyResult{}, nil
}
func (s *stubClient) GetDataSourceSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{}, nil
}
func (s *stubClient) ReadDataSource(context.Context, provider.ReadDataSourceRequest) (provider.ReadDataSourceResult, error) {
	return provider.ReadDataSourceResult{}, nil
}

func planNode() *ir.ResourceNode {
	return &ir.ResourceNode{Resource: ir.Resource{ID: "p.t.n", Provider: "p", Type: "t", Name: "n"}}
}

// No prior state => create, and the backend is sent a nil prior.
func TestPlanOpCreateWhenNoPrior(t *testing.T) {
	c := &stubClient{}
	res, err := Plan(context.Background(), c, provider.ResourceSchema{}, planNode(), map[string]interface{}{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Op != OpCreate {
		t.Errorf("Op = %v, want OpCreate", res.Op)
	}
	if c.gotPrior != nil {
		t.Errorf("create must send nil prior, got %#v", c.gotPrior)
	}
}

// Prior state + no RequiresReplace => update in place, and prior is sent through.
func TestPlanOpUpdateWhenPriorAndNoReplace(t *testing.T) {
	c := &stubClient{requiresReplace: false}
	prior := map[string]interface{}{"id": "existing"}
	res, err := Plan(context.Background(), c, provider.ResourceSchema{}, planNode(), map[string]interface{}{}, prior)
	if err != nil {
		t.Fatal(err)
	}
	if res.Op != OpUpdate {
		t.Errorf("Op = %v, want OpUpdate", res.Op)
	}
	if c.gotPrior["id"] != "existing" {
		t.Errorf("update must send the prior state, got %#v", c.gotPrior)
	}
}

// Prior state + RequiresReplace => replace.
func TestPlanOpReplaceWhenPriorAndReplace(t *testing.T) {
	c := &stubClient{requiresReplace: true}
	prior := map[string]interface{}{"id": "existing"}
	res, err := Plan(context.Background(), c, provider.ResourceSchema{}, planNode(), map[string]interface{}{}, prior)
	if err != nil {
		t.Fatal(err)
	}
	if res.Op != OpReplace {
		t.Errorf("Op = %v, want OpReplace", res.Op)
	}
}

// Prior state + the backend reports NoOp => OpNoop (nothing to do).
func TestPlanOpNoopWhenPriorAndNoChange(t *testing.T) {
	c := &stubClient{noop: true}
	prior := map[string]interface{}{"id": "existing"}
	res, err := Plan(context.Background(), c, provider.ResourceSchema{}, planNode(), map[string]interface{}{}, prior)
	if err != nil {
		t.Fatal(err)
	}
	if res.Op != OpNoop {
		t.Errorf("Op = %v, want OpNoop", res.Op)
	}
}

// RequiresReplace is meaningless for a create (no prior): it must not flip the op.
func TestPlanReplaceIgnoredWithoutPrior(t *testing.T) {
	c := &stubClient{requiresReplace: true}
	res, err := Plan(context.Background(), c, provider.ResourceSchema{}, planNode(), map[string]interface{}{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Op != OpCreate {
		t.Errorf("Op = %v, want OpCreate (replace is ignored without prior)", res.Op)
	}
}

// The human summary reflects the operation.
func TestRenderPlanWording(t *testing.T) {
	cases := []struct {
		op   Op
		want string
	}{
		{OpCreate, "will be created"},
		{OpUpdate, "will be updated in place"},
		{OpReplace, "will be replaced (destroy and re-create)"},
		{OpNoop, "is up to date (no change)"},
	}
	for _, tc := range cases {
		got := renderPlan("p.t.n", tc.op, nil)
		if got == "" || !contains(got, tc.want) {
			t.Errorf("renderPlan(op=%v) = %q, want it to contain %q", tc.op, got, tc.want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
