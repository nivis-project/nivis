// Copyright 2026 WeareTechnative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/wearetechnative/nivis/internal/plugin"
	"github.com/wearetechnative/nivis/internal/provider"
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
