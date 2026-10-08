// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package fakeproviderv5_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/nivis-project/nivis/internal/fakeproviderv5"
)

var contractType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
	"label": tftypes.String,
	"tier":  tftypes.String,
	"zone":  tftypes.String,
	"id":    tftypes.String,
}}

func dynamic(t *testing.T, vals map[string]tftypes.Value) *tfprotov5.DynamicValue {
	t.Helper()
	dv, err := tfprotov5.NewDynamicValue(contractType, tftypes.NewValue(contractType, vals))
	if err != nil {
		t.Fatalf("NewDynamicValue: %v", err)
	}
	return &dv
}

func decodeState(t *testing.T, dv *tfprotov5.DynamicValue) map[string]tftypes.Value {
	t.Helper()
	v, err := dv.Unmarshal(contractType)
	if err != nil {
		t.Fatalf("unmarshal state: %v", err)
	}
	m := map[string]tftypes.Value{}
	if err := v.As(&m); err != nil {
		t.Fatalf("state.As: %v", err)
	}
	return m
}

func str(t *testing.T, v tftypes.Value) string {
	t.Helper()
	var s string
	if err := v.As(&s); err != nil {
		t.Fatalf("value.As string: %v", err)
	}
	return s
}

func noErr(t *testing.T, diags []*tfprotov5.Diagnostic) {
	t.Helper()
	for _, d := range diags {
		if d.Severity == tfprotov5.DiagnosticSeverityError {
			t.Fatalf("unexpected error diagnostic: %s / %s", d.Summary, d.Detail)
		}
	}
}

// contractResource carries the two shapes the plan-object contract turns on: an
// optional+computed attr with a declared default (tier) and one without (zone).
func contractResource() fakeproviderv5.Resource {
	return fakeproviderv5.Resource{
		TypeName: "contract_thing",
		Attrs: map[string]fakeproviderv5.Attr{
			"label": {Type: tftypes.String, Optional: true},
			"tier":  {Type: tftypes.String, Optional: true, Computed: true, Default: "standard"},
			"zone":  {Type: tftypes.String, Optional: true, Computed: true},
			"id":    {Type: tftypes.String, Computed: true},
		},
		Apply: func(in map[string]string, n int64) (map[string]string, []*tfprotov5.Diagnostic) {
			return map[string]string{
				"id":   fmt.Sprintf("contract-%d", n),
				"zone": fmt.Sprintf("zone-%d", n),
			}, nil
		},
	}
}

func contractServer(t *testing.T) *fakeproviderv5.Server {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	return fakeproviderv5.New(contractResource())
}

func unsetCfg() map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"label": tftypes.NewValue(tftypes.String, "L"),
		"tier":  tftypes.NewValue(tftypes.String, nil),
		"zone":  tftypes.NewValue(tftypes.String, nil),
		"id":    tftypes.NewValue(tftypes.String, nil),
	}
}

func contractPlan(t *testing.T, cfg, proposed map[string]tftypes.Value) map[string]tftypes.Value {
	t.Helper()
	plan, err := contractServer(t).PlanResourceChange(context.Background(), &tfprotov5.PlanResourceChangeRequest{
		TypeName:         "contract_thing",
		Config:           dynamic(t, cfg),
		ProposedNewState: dynamic(t, proposed),
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, plan.Diagnostics)
	return decodeState(t, plan.PlannedState)
}

// TestPlannedStateComesFromProposedNewState is the discriminating test: config
// and proposed disagree about zone, and only a provider reading proposed can
// return the prior value. An implementation reading Config plans unknown here.
func TestPlannedStateComesFromProposedNewState(t *testing.T) {
	proposed := unsetCfg()
	proposed["zone"] = tftypes.NewValue(tftypes.String, "zone-7")
	proposed["id"] = tftypes.NewValue(tftypes.String, "contract-7")

	planned := contractPlan(t, unsetCfg(), proposed)

	if !planned["zone"].IsKnown() {
		t.Fatalf("zone unknown at plan: the planned state was built from Config, not ProposedNewState")
	}
	if got := str(t, planned["zone"]); got != "zone-7" {
		t.Errorf("zone = %q, want zone-7 (the prior value carried in by proposed)", got)
	}
}

func TestOptionalComputedUnsetWithNoPriorIsUnknown(t *testing.T) {
	planned := contractPlan(t, unsetCfg(), unsetCfg())

	if planned["zone"].IsKnown() {
		t.Errorf("zone = %v, want unknown: unset, no prior and no default", planned["zone"])
	}
	if planned["id"].IsKnown() {
		t.Errorf("id = %v, want unknown: computed-only attrs stay unknown at plan", planned["id"])
	}
}

func TestOptionalComputedSetInConfigWins(t *testing.T) {
	cfg := unsetCfg()
	cfg["zone"] = tftypes.NewValue(tftypes.String, "eu-central")
	proposed := unsetCfg()
	proposed["zone"] = tftypes.NewValue(tftypes.String, "eu-central")

	if got := str(t, contractPlan(t, cfg, proposed)["zone"]); got != "eu-central" {
		t.Errorf("zone = %q, want eu-central", got)
	}
}

func TestDeclaredDefaultFiresWhenConfigIsNull(t *testing.T) {
	ctx := context.Background()
	srv := contractServer(t)
	config := dynamic(t, unsetCfg())

	plan, err := srv.PlanResourceChange(ctx, &tfprotov5.PlanResourceChangeRequest{
		TypeName: "contract_thing", Config: config, ProposedNewState: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, plan.Diagnostics)
	planned := decodeState(t, plan.PlannedState)
	if !planned["tier"].IsKnown() {
		t.Fatalf("tier unknown at plan; the declared default never fired")
	}
	if got := str(t, planned["tier"]); got != "standard" {
		t.Fatalf("tier = %q at plan, want the declared default standard", got)
	}

	apply, err := srv.ApplyResourceChange(ctx, &tfprotov5.ApplyResourceChangeRequest{
		TypeName: "contract_thing", Config: config, PlannedState: plan.PlannedState,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, apply.Diagnostics)
	if got := str(t, decodeState(t, apply.NewState)["tier"]); got != "standard" {
		t.Errorf("tier = %q after apply, want standard: the default did not survive", got)
	}
}

func TestConfiguredValueBeatsTheDefault(t *testing.T) {
	ctx := context.Background()
	srv := contractServer(t)
	cfg := unsetCfg()
	cfg["tier"] = tftypes.NewValue(tftypes.String, "premium")
	config := dynamic(t, cfg)

	plan, err := srv.PlanResourceChange(ctx, &tfprotov5.PlanResourceChangeRequest{
		TypeName: "contract_thing", Config: config, ProposedNewState: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, plan.Diagnostics)
	if got := str(t, decodeState(t, plan.PlannedState)["tier"]); got != "premium" {
		t.Fatalf("tier = %q at plan, want premium", got)
	}

	apply, err := srv.ApplyResourceChange(ctx, &tfprotov5.ApplyResourceChangeRequest{
		TypeName: "contract_thing", Config: config, PlannedState: plan.PlannedState,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, apply.Diagnostics)
	if got := str(t, decodeState(t, apply.NewState)["tier"]); got != "premium" {
		t.Errorf("tier = %q after apply, want premium", got)
	}
}

// A prior value outranks the default: an existing resource is not re-defaulted.
func TestPriorValueBeatsTheDefault(t *testing.T) {
	proposed := unsetCfg()
	proposed["tier"] = tftypes.NewValue(tftypes.String, "legacy")

	if got := str(t, contractPlan(t, unsetCfg(), proposed)["tier"]); got != "legacy" {
		t.Errorf("tier = %q, want legacy: a prior value must outrank the default", got)
	}
}
