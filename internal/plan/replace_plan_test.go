// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/provider"
)

// recordingClient records every Plan call, so a test can assert how many were
// made and with which prior. Like the real backends it reports requires-replace
// only when there is a prior, and it returns a distinguishable planned state per
// call so the caller cannot pass by accident.
type recordingClient struct {
	requiresReplace bool
	priors          []map[string]interface{}
	failSecond      bool
}

func (c *recordingClient) Configure(context.Context, map[string]interface{}) error { return nil }
func (c *recordingClient) ListResourceTypes(context.Context) ([]string, error)     { return nil, nil }
func (c *recordingClient) GetSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{}, nil
}

func (c *recordingClient) Plan(_ context.Context, req provider.PlanRequest) (provider.PlanResult, error) {
	c.priors = append(c.priors, req.Prior)
	if c.failSecond && len(c.priors) == 2 {
		return provider.PlanResult{}, errors.New("provider exploded")
	}
	if req.Prior == nil {
		return provider.PlanResult{
			PlannedState:      "planned-from-null",
			UnknownAfterApply: []string{"group"},
		}, nil
	}
	return provider.PlanResult{
		PlannedState:      "planned-from-prior",
		UnknownAfterApply: nil,
		RequiresReplace:   c.requiresReplace,
	}, nil
}

func (c *recordingClient) Apply(context.Context, provider.ApplyRequest) (provider.ApplyResult, error) {
	return provider.ApplyResult{}, nil
}
func (c *recordingClient) Read(context.Context, provider.ReadRequest) (provider.ReadResult, error) {
	return provider.ReadResult{}, nil
}
func (c *recordingClient) Destroy(context.Context, provider.DestroyRequest) (provider.DestroyResult, error) {
	return provider.DestroyResult{}, nil
}
func (c *recordingClient) GetDataSourceSchema(context.Context, string) (provider.ResourceSchema, error) {
	return provider.ResourceSchema{}, nil
}
func (c *recordingClient) ReadDataSource(context.Context, provider.ReadDataSourceRequest) (provider.ReadDataSourceResult, error) {
	return provider.ReadDataSourceResult{}, nil
}

func existingPrior() map[string]interface{} {
	return map[string]interface{}{"id": "existing", "group": "inherited"}
}

// TestReplacePlansTheCreateHalfAgainstNull is the regression for bean
// nixform2-aur6: the object holding the prior computed values is about to be
// destroyed, so the planned state handed to apply must come from a plan made
// against a null prior.
func TestReplacePlansTheCreateHalfAgainstNull(t *testing.T) {
	c := &recordingClient{requiresReplace: true}

	res, err := Plan(context.Background(), c, provider.ResourceSchema{}, planNode(), map[string]interface{}{}, existingPrior())
	if err != nil {
		t.Fatal(err)
	}

	if len(c.priors) != 2 {
		t.Fatalf("a replacement must be planned twice, got %d call(s)", len(c.priors))
	}
	if c.priors[0] == nil {
		t.Errorf("the first plan must carry the real prior; requires-replace cannot be detected without it")
	}
	if c.priors[1] != nil {
		t.Errorf("the second plan must carry a nil prior, got %#v", c.priors[1])
	}
	if res.PlannedState != "planned-from-null" {
		t.Errorf("PlannedState = %v, want the second plan's: the create half must not inherit from the destroyed object", res.PlannedState)
	}
}

// The operation still comes from the first plan: only a plan against the real
// prior can report requires-replace.
func TestReplaceStillReportsAReplacement(t *testing.T) {
	c := &recordingClient{requiresReplace: true}

	res, err := Plan(context.Background(), c, provider.ResourceSchema{}, planNode(), map[string]interface{}{}, existingPrior())
	if err != nil {
		t.Fatal(err)
	}
	if res.Op != OpReplace {
		t.Errorf("Op = %v, want OpReplace", res.Op)
	}
}

// What the replacement will recompute is reported from the create half, so the
// plan stops claiming a provider-filled attribute is already known.
func TestReplaceReportsWhatTheCreateHalfRecomputes(t *testing.T) {
	c := &recordingClient{requiresReplace: true}

	res, err := Plan(context.Background(), c, provider.ResourceSchema{}, planNode(), map[string]interface{}{}, existingPrior())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.UnknownAfterApply) != 1 || res.UnknownAfterApply[0] != "group" {
		t.Errorf("UnknownAfterApply = %v, want [group] from the null-prior plan", res.UnknownAfterApply)
	}
	if want := "p.t.n will be replaced (destroy and re-create) (group known after apply)"; res.Human != want {
		t.Errorf("Human = %q, want %q", res.Human, want)
	}
}

// Only a replacement plans twice. A create, an update and a no-op each make one
// call, so the extra round trip is paid only where it is needed.
func TestOnlyAReplacementPlansTwice(t *testing.T) {
	cases := []struct {
		name            string
		prior           map[string]interface{}
		requiresReplace bool
		wantCalls       int
		wantOp          Op
	}{
		{"create", nil, false, 1, OpCreate},
		{"update", existingPrior(), false, 1, OpUpdate},
		{"replace", existingPrior(), true, 2, OpReplace},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &recordingClient{requiresReplace: tc.requiresReplace}
			res, err := Plan(context.Background(), c, provider.ResourceSchema{}, planNode(), map[string]interface{}{}, tc.prior)
			if err != nil {
				t.Fatal(err)
			}
			if len(c.priors) != tc.wantCalls {
				t.Errorf("plan calls = %d, want %d", len(c.priors), tc.wantCalls)
			}
			if res.Op != tc.wantOp {
				t.Errorf("Op = %v, want %v", res.Op, tc.wantOp)
			}
		})
	}
}

// An update must still see its prior merged in, so a provider does not report a
// phantom change. The fix must not reach beyond replacements.
func TestUpdateStillPlansAgainstItsPrior(t *testing.T) {
	c := &recordingClient{requiresReplace: false}

	res, err := Plan(context.Background(), c, provider.ResourceSchema{}, planNode(), map[string]interface{}{}, existingPrior())
	if err != nil {
		t.Fatal(err)
	}
	if res.PlannedState != "planned-from-prior" {
		t.Errorf("PlannedState = %v, want the prior-carrying plan for an update", res.PlannedState)
	}
}

// A failure planning the create half is an error, not a silent fall back to the
// prior-carrying plan. It happens before anything is destroyed.
func TestReplaceSurfacesAFailureInTheSecondPlan(t *testing.T) {
	c := &recordingClient{requiresReplace: true, failSecond: true}

	_, err := Plan(context.Background(), c, provider.ResourceSchema{}, planNode(), map[string]interface{}{}, existingPrior())
	if err == nil {
		t.Fatal("expected an error when the create half fails to plan")
	}
	if got := err.Error(); !strings.Contains(got, "create half") || !strings.Contains(got, "p.t.n") {
		t.Errorf("error = %q, want it to name the create half and the resource", got)
	}
}

// UpgradeState satisfies provider.Client. These stubs model resources whose
// schema never changed, so an upgrade is the identity.
func (c *recordingClient) UpgradeState(_ context.Context, req provider.UpgradeStateRequest) (provider.UpgradeStateResult, error) {
	return provider.UpgradeStateResult{Attrs: req.Stored}, nil
}

// Import satisfies provider.Client. These stubs are not used for import tests.
func (c *recordingClient) Import(context.Context, provider.ImportRequest) (provider.ImportResult, error) {
	return provider.ImportResult{}, nil
}
