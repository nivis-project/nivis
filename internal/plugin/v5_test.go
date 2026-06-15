// Copyright 2026 WeareTechnative B.V. and the nixform authors
// SPDX-License-Identifier: Apache-2.0

package plugin_test

import (
	"context"
	"testing"

	"github.com/wearetechnative/nixform/internal/plugin"
	"github.com/wearetechnative/nixform/internal/provider"
)

// TestV5NegotiationAndRoundTrip spawns the REAL fake v5 provider (provider-gamma)
// through the manager and proves: (a) the manager negotiates protocol 5 and
// returns a working provider.Client, and (b) the full plan/apply round trip
// yields the exact deterministic outputs — identical handling to a v6 provider.
// (buildProvider, repoRoot, contains are shared with integration_test.go.)
func TestV5NegotiationAndRoundTrip(t *testing.T) {
	bin := buildProvider(t, "provider-gamma")
	t.Setenv("NIXFORM_FAKE_COUNTER", "")
	ctx := context.Background()

	mgr := plugin.NewManager()
	defer mgr.Close()

	client, err := mgr.Client("gamma", bin)
	if err != nil {
		t.Fatalf("spawn/handshake (v5): %v", err)
	}

	types, err := client.ListResourceTypes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(types) != 1 || types[0] != "gamma_widget" {
		t.Fatalf("resource types = %v, want [gamma_widget]", types)
	}
	rs, err := client.GetSchema(ctx, "gamma_widget")
	if err != nil {
		t.Fatal(err)
	}

	pr, err := client.Plan(ctx, provider.PlanRequest{
		Schema: rs, TypeName: "gamma_widget",
		ResolvedCfg: map[string]interface{}{"size": "big"},
	})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !contains(pr.UnknownAfterApply, "id") || !contains(pr.UnknownAfterApply, "result") {
		t.Fatalf("plan should mark id+result unknown-after-apply; got %v", pr.UnknownAfterApply)
	}

	ar, err := client.Apply(ctx, provider.ApplyRequest{
		Schema: rs, TypeName: "gamma_widget",
		ResolvedCfg:  map[string]interface{}{"size": "big"},
		PlannedState: pr.PlannedState,
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if ar.Attrs["id"] != "gamma-0" {
		t.Errorf("id = %v, want gamma-0", ar.Attrs["id"])
	}
	if ar.Attrs["result"] != "widget:big:0" {
		t.Errorf("result = %v, want widget:big:0", ar.Attrs["result"])
	}
}
