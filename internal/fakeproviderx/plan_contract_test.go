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
			"zone":  {Type: tftypes.String, Optional: true, Computed: true, ForceNew: true},
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

// priorAttrs is a stored object: like a config, but with the provider-filled
// computed attr carrying the value the provider chose. A computed attr is not
// null in state, and a fixture that leaves it null makes an unknown planned
// value differ from it for reasons the test is not about.
func priorAttrs(image, label string) map[string]tftypes.Value {
	p := replaceAttrs(image, label)
	p["zone"] = tftypes.NewValue(tftypes.String, "zone-0")
	return p
}

// planReplace plans cfg against prior; a nil prior models a create.
func planReplace(t *testing.T, cfg map[string]tftypes.Value, prior map[string]tftypes.Value) *tfprotov6.PlanResourceChangeResponse {
	t.Helper()
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	req := &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "replace_thing",
		Config:           replaceDynamic(t, cfg),
		ProposedNewState: replaceDynamic(t, proposedFrom(cfg, prior)),
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
	prior := priorAttrs("ami-old", "L")

	resp := planReplace(t, replaceAttrs("ami-new", "L"), prior)

	if len(resp.RequiresReplace) != 1 {
		t.Fatalf("RequiresReplace = %v, want exactly the image attribute", resp.RequiresReplace)
	}
	if got := resp.RequiresReplace[0].String(); got != "AttributeName(\"image\")" {
		t.Errorf("RequiresReplace[0] = %s, want the image attribute", got)
	}
}

func TestUnchangedForceNewAttrDoesNotRequireReplace(t *testing.T) {
	if resp := planReplace(t, replaceAttrs("ami-old", "L"), priorAttrs("ami-old", "L")); len(resp.RequiresReplace) != 0 {
		t.Errorf("RequiresReplace = %v, want empty: the force-new attr did not change", resp.RequiresReplace)
	}
}

func TestChangedOrdinaryAttrDoesNotRequireReplace(t *testing.T) {
	if resp := planReplace(t, replaceAttrs("ami-old", "NEW"), priorAttrs("ami-old", "L")); len(resp.RequiresReplace) != 0 {
		t.Errorf("RequiresReplace = %v, want empty: label is not force-new", resp.RequiresReplace)
	}
}

// A create has no prior to differ from, so it can never require replacement.
func TestCreateNeverRequiresReplace(t *testing.T) {
	if resp := planReplace(t, replaceAttrs("ami-new", "L"), nil); len(resp.RequiresReplace) != 0 {
		t.Errorf("RequiresReplace = %v, want empty on a create", resp.RequiresReplace)
	}
}

// A force-new attr whose planned value is UNKNOWN must replace: unknown differs
// from a known prior. This is what a cascade through an unresolved reference
// depends on, and it is what terraform-plugin-framework does
// (!PlanValue.Equal(StateValue)) and SDKv2 does (RequiresNew on NewComputed).
func TestUnknownForceNewAttrRequiresReplace(t *testing.T) {
	prior := priorAttrs("ami-old", "L")
	cfg := replaceAttrs("ami-old", "L")
	cfg["image"] = tftypes.NewValue(tftypes.String, tftypes.UnknownValue)

	if resp := planReplace(t, cfg, prior); len(resp.RequiresReplace) != 1 {
		t.Errorf("RequiresReplace = %v, want the image attr: an unknown planned value differs from a known prior", resp.RequiresReplace)
	}
}

// A force-new attr REMOVED from config plans as null against a known prior, so
// it must replace. The old rule skipped null config and missed this.
func TestRemovedForceNewAttrRequiresReplace(t *testing.T) {
	prior := priorAttrs("ami-old", "L")
	cfg := replaceAttrs("ami-old", "L")
	cfg["image"] = tftypes.NewValue(tftypes.String, nil)

	if resp := planReplace(t, cfg, prior); len(resp.RequiresReplace) != 1 {
		t.Errorf("RequiresReplace = %v, want the image attr: removing a force-new attr is a change", resp.RequiresReplace)
	}
}

// The case the deleted scenario was really protecting: an optional+computed
// force-new attr the config never sets does NOT replace, because its planned
// value carries the prior forward and the two compare equal. No exclusion is
// needed for this; comparing the planned value is enough.
func TestUnsetOptionalComputedForceNewDoesNotRequireReplace(t *testing.T) {
	prior := priorAttrs("ami-old", "L")
	cfg := replaceAttrs("ami-old", "L") // zone absent from config

	if resp := planReplace(t, cfg, prior); len(resp.RequiresReplace) != 0 {
		t.Errorf("RequiresReplace = %v, want empty: zone is carried forward, not changed", resp.RequiresReplace)
	}
}

// proposedFrom models what the executor sends as ProposedNewState: the config,
// with a prior value merged into any computed attr the config does not set.
// Without that merge an unset optional-computed attr would plan as unknown and
// look like a change on every plan, which is not what a provider sees.
func proposedFrom(cfg, prior map[string]tftypes.Value) map[string]tftypes.Value {
	out := map[string]tftypes.Value{}
	for k, v := range cfg {
		out[k] = v
	}
	if prior == nil {
		return out
	}
	for _, name := range []string{"zone", "id"} {
		cv, ok := out[name]
		if ok && cv.IsKnown() && !cv.IsNull() {
			continue
		}
		if pv, ok := prior[name]; ok && pv.IsKnown() && !pv.IsNull() {
			out[name] = pv
		}
	}
	return out
}

// Failure injection exists so the executor's failure paths can be reached
// through a spawned provider. See internal/fakeprovider for the rationale.
func TestInjectedApplyFailure(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_FAIL", "apply:replace_thing")
	srv := fakeproviderx.New(replaceResource())
	config := replaceDynamic(t, replaceAttrs("ami-old", "L"))

	for i := range 2 {
		resp, err := srv.ApplyResourceChange(context.Background(), &tfprotov6.ApplyResourceChangeRequest{
			TypeName: "replace_thing", Config: config, PlannedState: config,
		})
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if len(resp.Diagnostics) == 0 || resp.Diagnostics[0].Severity != tfprotov6.DiagnosticSeverityError {
			t.Fatalf("call %d: want an error diagnostic, got %v", i, resp.Diagnostics)
		}
	}
}

func TestInjectionIsOffByDefault(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_FAIL", "")
	srv := fakeproviderx.New(replaceResource())
	config := replaceDynamic(t, replaceAttrs("ami-old", "L"))

	resp, err := srv.ApplyResourceChange(context.Background(), &tfprotov6.ApplyResourceChangeRequest{
		TypeName: "replace_thing", Config: config, PlannedState: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("unexpected error diagnostic: %s", d.Summary)
		}
	}
}

func TestInjectionIsAddressedByOperationAndType(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_FAIL", "apply:other_thing")
	srv := fakeproviderx.New(replaceResource())
	config := replaceDynamic(t, replaceAttrs("ami-old", "L"))

	resp, err := srv.ApplyResourceChange(context.Background(), &tfprotov6.ApplyResourceChangeRequest{
		TypeName: "replace_thing", Config: config, PlannedState: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("unexpected error diagnostic: %s", d.Summary)
		}
	}
}
