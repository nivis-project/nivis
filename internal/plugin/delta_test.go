// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/nivis-project/nivis/internal/ir"
	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/provider"
)

// TestDeltaCollectionsRoundTrip drives the REAL provider-delta binary through
// the manager and proves collection (map/list) inputs and list/object computed
// outputs round-trip through the full encode -> provider -> decode pipeline.
// (buildProvider/contains are shared with integration_test.go.)
func TestDeltaCollectionsRoundTrip(t *testing.T) {
	bin := buildProvider(t, "provider-delta")
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	ctx := context.Background()

	mgr := plugin.NewManager()
	defer mgr.Close()

	client, err := mgr.Client("delta", bin, map[string]interface{}{})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	rs, err := client.GetSchema(ctx, "delta_thing")
	if err != nil {
		t.Fatal(err)
	}

	cfg := map[string]interface{}{
		"tags":  map[string]interface{}{"env": "prod"},
		"ports": []interface{}{float64(80), float64(443)},
	}

	// Plan: computed collection/object attrs unknown.
	pr, err := client.Plan(ctx, provider.PlanRequest{Schema: rs, TypeName: "delta_thing", ResolvedCfg: cfg})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, want := range []string{"id", "endpoints", "meta"} {
		if !contains(pr.UnknownAfterApply, want) {
			t.Errorf("expected %q unknown at plan; got %v", want, pr.UnknownAfterApply)
		}
	}

	// Apply: collection/object outputs become concrete and round-trip.
	ar, err := client.Apply(ctx, provider.ApplyRequest{
		Schema: rs, TypeName: "delta_thing", ResolvedCfg: cfg, PlannedState: pr.PlannedState,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	// endpoints: one "ep-<port>-0" per input port (counter 0).
	wantEndpoints := []interface{}{"ep-80-0", "ep-443-0"}
	if got := ar.Attrs["endpoints"]; !reflect.DeepEqual(got, wantEndpoints) {
		t.Errorf("endpoints = %#v, want %#v", got, wantEndpoints)
	}
	// meta: object derived from tags — region=prod, count=1.
	meta, ok := ar.Attrs["meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("meta is not an object: %#v", ar.Attrs["meta"])
	}
	if meta["region"] != "prod" {
		t.Errorf("meta.region = %v, want prod", meta["region"])
	}
	if meta["count"] != float64(1) {
		t.Errorf("meta.count = %v, want 1", meta["count"])
	}
	if ar.Attrs["id"] != "delta-0" {
		t.Errorf("id = %v, want delta-0", ar.Attrs["id"])
	}
}

// TestDeltaDefaultReachesApply is the integration-level proof for bean
// nixform2-1mk0. tier is optional+computed with a declared default and is absent
// from config, so EncodeConfig sends null, the provider applies its default at
// plan, and the value survives into the applied state. Before the encoder sent
// null this test's predecessor asserted the defect: tier stayed unknown through
// plan and arrived empty at apply.
func TestDeltaDefaultReachesApply(t *testing.T) {
	bin := buildProvider(t, "provider-delta")
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	ctx := context.Background()

	mgr := plugin.NewManager()
	defer mgr.Close()

	client, err := mgr.Client("delta", bin, map[string]interface{}{})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	rs, err := client.GetSchema(ctx, "delta_thing")
	if err != nil {
		t.Fatal(err)
	}

	cfg := map[string]interface{}{"label": "L"}

	pr, err := client.Plan(ctx, provider.PlanRequest{Schema: rs, TypeName: "delta_thing", ResolvedCfg: cfg})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if contains(pr.UnknownAfterApply, "tier") {
		t.Fatalf("tier is unknown at plan: the declared default did not fire; got unknowns %v", pr.UnknownAfterApply)
	}
	// The computed-only attrs must still be unknown: the default rule must not
	// have leaked into attributes the user cannot set.
	for _, want := range []string{"id", "endpoints", "meta"} {
		if !contains(pr.UnknownAfterApply, want) {
			t.Errorf("expected %q unknown at plan; got %v", want, pr.UnknownAfterApply)
		}
	}

	ar, err := client.Apply(ctx, provider.ApplyRequest{
		Schema: rs, TypeName: "delta_thing", ResolvedCfg: cfg, PlannedState: pr.PlannedState,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := ar.Attrs["tier"]; got != "standard" {
		t.Errorf("tier = %#v after apply, want \"standard\": the default did not survive", got)
	}
}

// TestDeltaConfiguredValueBeatsDefault is the other half: a value the user set
// must reach the provider unchanged rather than being replaced by the default.
func TestDeltaConfiguredValueBeatsDefault(t *testing.T) {
	bin := buildProvider(t, "provider-delta")
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	ctx := context.Background()

	mgr := plugin.NewManager()
	defer mgr.Close()

	client, err := mgr.Client("delta", bin, map[string]interface{}{})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	rs, err := client.GetSchema(ctx, "delta_thing")
	if err != nil {
		t.Fatal(err)
	}

	cfg := map[string]interface{}{"label": "L", "tier": "premium"}

	pr, err := client.Plan(ctx, provider.PlanRequest{Schema: rs, TypeName: "delta_thing", ResolvedCfg: cfg})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	ar, err := client.Apply(ctx, provider.ApplyRequest{
		Schema: rs, TypeName: "delta_thing", ResolvedCfg: cfg, PlannedState: pr.PlannedState,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := ar.Attrs["tier"]; got != "premium" {
		t.Errorf("tier = %#v after apply, want \"premium\"", got)
	}
}

// TestDeltaForceNewInputIsReplaced drives the REAL provider-delta binary and
// proves a replacement is reachable through the protocol: apply once, change the
// force-new input, and the provider reports requires-replace.
//
// It also pins the CLIENT-layer contract, which is correct and must not change:
// at this layer an unset computed attr takes its prior value, because that is
// what keeps an in-place update from showing a phantom diff. The
// replacement-specific correction happens one layer up, where the operation is
// known. See TestDeltaReplaceDoesNotInheritFromTheDestroyedObject.
func TestDeltaForceNewInputIsReplaced(t *testing.T) {
	bin := buildProvider(t, "provider-delta")
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	ctx := context.Background()

	mgr := plugin.NewManager()
	defer mgr.Close()

	client, err := mgr.Client("delta", bin, map[string]interface{}{})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	rs, err := client.GetSchema(ctx, "delta_thing")
	if err != nil {
		t.Fatal(err)
	}

	// Create once, so `group` lands in state with a provider-chosen value the
	// configuration never set.
	cfg := map[string]interface{}{"label": "L", "image": "ami-old"}
	pr, err := client.Plan(ctx, provider.PlanRequest{Schema: rs, TypeName: "delta_thing", ResolvedCfg: cfg})
	if err != nil {
		t.Fatalf("plan create: %v", err)
	}
	ar, err := client.Apply(ctx, provider.ApplyRequest{
		Schema: rs, TypeName: "delta_thing", ResolvedCfg: cfg, PlannedState: pr.PlannedState,
	})
	if err != nil {
		t.Fatalf("apply create: %v", err)
	}
	group, _ := ar.Attrs["group"].(string)
	if group == "" {
		t.Fatalf("group is empty after create; the provider should have filled it: %#v", ar.Attrs)
	}

	// Change the force-new input, planning against the stored object.
	changed := map[string]interface{}{"label": "L", "image": "ami-new"}
	rp, err := client.Plan(ctx, provider.PlanRequest{
		Schema: rs, TypeName: "delta_thing", ResolvedCfg: changed, Prior: ar.Attrs,
	})
	if err != nil {
		t.Fatalf("plan replace: %v", err)
	}
	if !rp.RequiresReplace {
		t.Fatalf("expected a replacement when the force-new input changed; got unknowns %v", rp.UnknownAfterApply)
	}

	// At this layer `group` keeps its prior value, which is the update contract.
	// Deciding that a replacement should not inherit it needs the operation, which
	// only the plan engine knows.
	if contains(rp.UnknownAfterApply, "group") {
		t.Errorf("group is unknown at the client layer; an unset computed attr must take its "+
			"prior value here, or an in-place update shows a phantom diff. Got %v", rp.UnknownAfterApply)
	}
}

// TestDeltaUnchangedForceNewInputIsNotReplaced guards the other direction: the
// force-new marker must not turn every plan of an existing resource into a
// replacement.
func TestDeltaUnchangedForceNewInputIsNotReplaced(t *testing.T) {
	bin := buildProvider(t, "provider-delta")
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	ctx := context.Background()

	mgr := plugin.NewManager()
	defer mgr.Close()

	client, err := mgr.Client("delta", bin, map[string]interface{}{})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	rs, err := client.GetSchema(ctx, "delta_thing")
	if err != nil {
		t.Fatal(err)
	}

	cfg := map[string]interface{}{"label": "L", "image": "ami-old"}
	pr, err := client.Plan(ctx, provider.PlanRequest{Schema: rs, TypeName: "delta_thing", ResolvedCfg: cfg})
	if err != nil {
		t.Fatalf("plan create: %v", err)
	}
	ar, err := client.Apply(ctx, provider.ApplyRequest{
		Schema: rs, TypeName: "delta_thing", ResolvedCfg: cfg, PlannedState: pr.PlannedState,
	})
	if err != nil {
		t.Fatalf("apply create: %v", err)
	}

	rp, err := client.Plan(ctx, provider.PlanRequest{
		Schema: rs, TypeName: "delta_thing", ResolvedCfg: cfg, Prior: ar.Attrs,
	})
	if err != nil {
		t.Fatalf("plan again: %v", err)
	}
	if rp.RequiresReplace {
		t.Errorf("unchanged config planned as a replacement")
	}
}

// TestDeltaReplaceDoesNotInheritFromTheDestroyedObject is the integration proof
// for bean nixform2-aur6. It goes through plan.Plan, which is where the
// operation is decided and therefore where the create half of a replacement is
// planned against a null prior. Driving client.Plan directly would not show it:
// at that layer carrying prior values into unset computed attrs is correct, and
// is what keeps an in-place update from showing a phantom diff.
func TestDeltaReplaceDoesNotInheritFromTheDestroyedObject(t *testing.T) {
	bin := buildProvider(t, "provider-delta")
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	ctx := context.Background()

	mgr := plugin.NewManager()
	defer mgr.Close()

	client, err := mgr.Client("delta", bin, map[string]interface{}{})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	rs, err := client.GetSchema(ctx, "delta_thing")
	if err != nil {
		t.Fatal(err)
	}
	node := &ir.ResourceNode{Resource: ir.Resource{
		ID: "delta.delta_thing.t", Provider: "delta", Type: "delta_thing", Name: "t",
	}}

	// Create once so `group` lands in state with a provider-chosen value the
	// configuration never set. This is the AWS `security_groups` shape.
	cfg := map[string]interface{}{"label": "L", "image": "ami-old"}
	pr, err := plan.Plan(ctx, client, rs, node, cfg, nil)
	if err != nil {
		t.Fatalf("plan create: %v", err)
	}
	ar, err := client.Apply(ctx, provider.ApplyRequest{
		Schema: rs, TypeName: "delta_thing", ResolvedCfg: cfg, PlannedState: pr.PlannedState,
	})
	if err != nil {
		t.Fatalf("apply create: %v", err)
	}
	group, _ := ar.Attrs["group"].(string)
	if group == "" {
		t.Fatalf("group empty after create; the provider should have filled it: %#v", ar.Attrs)
	}

	// Change the force-new input and plan against the stored object.
	changed := map[string]interface{}{"label": "L", "image": "ami-new"}
	rp, err := plan.Plan(ctx, client, rs, node, changed, ar.Attrs)
	if err != nil {
		t.Fatalf("plan replace: %v", err)
	}
	if rp.Op != plan.OpReplace {
		t.Fatalf("Op = %v, want a replacement", rp.Op)
	}
	if !contains(rp.UnknownAfterApply, "group") {
		t.Errorf("group is not known-after-apply: the create half inherited %q from the object "+
			"about to be destroyed; got unknowns %v", group, rp.UnknownAfterApply)
	}

	// Applying the create half must not receive the inherited value either.
	na, err := client.Apply(ctx, provider.ApplyRequest{
		Schema: rs, TypeName: "delta_thing", ResolvedCfg: changed, PlannedState: rp.PlannedState,
	})
	if err != nil {
		t.Fatalf("apply replace: %v", err)
	}
	if got, _ := na.Attrs["group"].(string); got == group {
		t.Errorf("group = %q after the replacement, the same value the destroyed object had", got)
	}
}
