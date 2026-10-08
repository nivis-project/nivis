// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"context"
	"reflect"
	"testing"

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

// TestDeltaDefaultBlockedByEncoderUnknown pins bean nixform2-1mk0 at the
// integration level. The fake now applies a declared default only where config
// is null, as terraform-plugin-framework does. tfvalue.EncodeConfig still sends
// unknown for an attribute absent from config but computed in the schema, and
// unknown is not null, so the default does not fire and tier reaches apply
// empty. That is the defect, reproduced through a real spawn.
//
// The next change makes EncodeConfig send null. When it lands this test INVERTS:
// tier must then be "standard" at plan and after apply. Failing here is the
// signal to update it, not to loosen the provider.
func TestDeltaDefaultBlockedByEncoderUnknown(t *testing.T) {
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
	if !contains(pr.UnknownAfterApply, "tier") {
		t.Fatalf("tier is NOT unknown at plan: EncodeConfig now sends null and the default fires. "+
			"nixform2-1mk0 is fixed, so invert this test (want tier=standard). Got unknowns %v", pr.UnknownAfterApply)
	}

	ar, err := client.Apply(ctx, provider.ApplyRequest{
		Schema: rs, TypeName: "delta_thing", ResolvedCfg: cfg, PlannedState: pr.PlannedState,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := ar.Attrs["tier"]; got != nil && got != "" {
		t.Fatalf("tier = %#v after apply; expected the defect (empty), so nixform2-1mk0 may be fixed: invert this test", got)
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
