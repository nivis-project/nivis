// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package fakeproviderx_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/nivis-project/nivis/internal/fakeproviderx"
)

var contractType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
	"label": tftypes.String,
	"tier":  tftypes.String,
	"zone":  tftypes.String,
	"id":    tftypes.String,
	"ports": tftypes.List{ElementType: tftypes.Number},
}}

func dynamic(t *testing.T, vals map[string]tftypes.Value) *tfprotov6.DynamicValue {
	t.Helper()
	dv, err := tfprotov6.NewDynamicValue(contractType, tftypes.NewValue(contractType, vals))
	if err != nil {
		t.Fatalf("NewDynamicValue: %v", err)
	}
	return &dv
}

func decodeState(t *testing.T, dv *tfprotov6.DynamicValue) map[string]tftypes.Value {
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

// contractResource mirrors the delta_thing shape: collection attrs alongside an
// optional+computed attr with a declared default (tier) and one without (zone).
func contractResource() fakeproviderx.Resource {
	return fakeproviderx.Resource{
		TypeName: "contract_thing",
		Attrs: map[string]fakeproviderx.Attr{
			"label": {Type: tftypes.String, Optional: true},
			"tier":  {Type: tftypes.String, Optional: true, Computed: true, Default: "standard"},
			"zone":  {Type: tftypes.String, Optional: true, Computed: true},
			"ports": {Type: tftypes.List{ElementType: tftypes.Number}, Optional: true},
			"id":    {Type: tftypes.String, Computed: true},
		},
		Apply: func(in map[string]interface{}, n int64) map[string]interface{} {
			return map[string]interface{}{
				"id":   fmt.Sprintf("contract-%d", n),
				"zone": fmt.Sprintf("zone-%d", n),
			}
		},
	}
}

func contractServer(t *testing.T) *fakeproviderx.Server {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	return fakeproviderx.New(contractResource())
}

func unsetCfg() map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"label": tftypes.NewValue(tftypes.String, "L"),
		"tier":  tftypes.NewValue(tftypes.String, nil),
		"zone":  tftypes.NewValue(tftypes.String, nil),
		"ports": tftypes.NewValue(tftypes.List{ElementType: tftypes.Number}, nil),
		"id":    tftypes.NewValue(tftypes.String, nil),
	}
}

func contractPlan(t *testing.T, cfg, proposed map[string]tftypes.Value) map[string]tftypes.Value {
	t.Helper()
	plan, err := contractServer(t).PlanResourceChange(context.Background(), &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "contract_thing",
		Config:           dynamic(t, cfg),
		ProposedNewState: dynamic(t, proposed),
	})
	if err != nil {
		t.Fatal(err)
	}
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

	plan, err := srv.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName: "contract_thing", Config: config, ProposedNewState: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	planned := decodeState(t, plan.PlannedState)
	if !planned["tier"].IsKnown() {
		t.Fatalf("tier unknown at plan; the declared default never fired")
	}
	if got := str(t, planned["tier"]); got != "standard" {
		t.Fatalf("tier = %q at plan, want the declared default standard", got)
	}

	apply, err := srv.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{
		TypeName: "contract_thing", Config: config, PlannedState: plan.PlannedState,
	})
	if err != nil {
		t.Fatal(err)
	}
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

	plan, err := srv.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName: "contract_thing", Config: config, ProposedNewState: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := str(t, decodeState(t, plan.PlannedState)["tier"]); got != "premium" {
		t.Fatalf("tier = %q at plan, want premium", got)
	}

	apply, err := srv.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{
		TypeName: "contract_thing", Config: config, PlannedState: plan.PlannedState,
	})
	if err != nil {
		t.Fatal(err)
	}
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

var replaceType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
	"image": tftypes.String,
	"label": tftypes.String,
	"zone":  tftypes.String,
	"id":    tftypes.String,
}}

func replaceDynamic(t *testing.T, vals map[string]tftypes.Value) *tfprotov6.DynamicValue {
	t.Helper()
	dv, err := tfprotov6.NewDynamicValue(replaceType, tftypes.NewValue(replaceType, vals))
	if err != nil {
		t.Fatalf("NewDynamicValue: %v", err)
	}
	return &dv
}

func replaceResource() fakeproviderx.Resource {
	return fakeproviderx.Resource{
		TypeName: "replace_thing",
		Attrs: map[string]fakeproviderx.Attr{
			"image": {Type: tftypes.String, Optional: true, ForceNew: true},
			"label": {Type: tftypes.String, Optional: true},
			"zone":  {Type: tftypes.String, Optional: true, Computed: true},
			"id":    {Type: tftypes.String, Computed: true},
		},
		Apply: func(_ map[string]interface{}, n int64) map[string]interface{} {
			return map[string]interface{}{
				"id":   fmt.Sprintf("replace-%d", n),
				"zone": fmt.Sprintf("zone-%d", n),
			}
		},
	}
}

func replaceAttrs(image, label string) map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"image": tftypes.NewValue(tftypes.String, image),
		"label": tftypes.NewValue(tftypes.String, label),
		"zone":  tftypes.NewValue(tftypes.String, nil),
		"id":    tftypes.NewValue(tftypes.String, nil),
	}
}

// planReplace plans cfg against prior; a nil prior models a create.
func planReplace(t *testing.T, cfg map[string]tftypes.Value, prior map[string]tftypes.Value) *tfprotov6.PlanResourceChangeResponse {
	t.Helper()
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	req := &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "replace_thing",
		Config:           replaceDynamic(t, cfg),
		ProposedNewState: replaceDynamic(t, cfg),
	}
	if prior != nil {
		req.PriorState = replaceDynamic(t, prior)
	}
	resp, err := fakeproviderx.New(replaceResource()).PlanResourceChange(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestChangedForceNewAttrRequiresReplace(t *testing.T) {
	prior := replaceAttrs("ami-old", "L")
	prior["zone"] = tftypes.NewValue(tftypes.String, "zone-0")

	resp := planReplace(t, replaceAttrs("ami-new", "L"), prior)

	if len(resp.RequiresReplace) != 1 {
		t.Fatalf("RequiresReplace = %v, want exactly the image attribute", resp.RequiresReplace)
	}
	if got := resp.RequiresReplace[0].String(); got != "AttributeName(\"image\")" {
		t.Errorf("RequiresReplace[0] = %s, want the image attribute", got)
	}
}

func TestUnchangedForceNewAttrDoesNotRequireReplace(t *testing.T) {
	if resp := planReplace(t, replaceAttrs("ami-old", "L"), replaceAttrs("ami-old", "L")); len(resp.RequiresReplace) != 0 {
		t.Errorf("RequiresReplace = %v, want empty: the force-new attr did not change", resp.RequiresReplace)
	}
}

func TestChangedOrdinaryAttrDoesNotRequireReplace(t *testing.T) {
	if resp := planReplace(t, replaceAttrs("ami-old", "NEW"), replaceAttrs("ami-old", "L")); len(resp.RequiresReplace) != 0 {
		t.Errorf("RequiresReplace = %v, want empty: label is not force-new", resp.RequiresReplace)
	}
}

// A create has no prior to differ from, so it can never require replacement.
func TestCreateNeverRequiresReplace(t *testing.T) {
	if resp := planReplace(t, replaceAttrs("ami-new", "L"), nil); len(resp.RequiresReplace) != 0 {
		t.Errorf("RequiresReplace = %v, want empty on a create", resp.RequiresReplace)
	}
}

// An attr absent from config is not a change. Were it treated as one, every
// unset optional force-new attr would force a replacement on every plan.
func TestAbsentForceNewAttrIsNotAChange(t *testing.T) {
	cfg := replaceAttrs("ami-old", "L")
	cfg["image"] = tftypes.NewValue(tftypes.String, nil)

	if resp := planReplace(t, cfg, replaceAttrs("ami-old", "L")); len(resp.RequiresReplace) != 0 {
		t.Errorf("RequiresReplace = %v, want empty: image is absent from config, not changed", resp.RequiresReplace)
	}
}
