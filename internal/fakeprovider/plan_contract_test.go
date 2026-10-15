// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package fakeprovider_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/nivis-project/nivis/internal/fakeprovider"
)

var contractType = objType(map[string]tftypes.Type{
	"label": tftypes.String,
	"tier":  tftypes.String,
	"zone":  tftypes.String,
	"id":    tftypes.String,
})

// contractResource carries the two shapes the plan-object contract turns on: an
// optional+computed attr with a declared default (tier) and one without (zone).
func contractResource() fakeprovider.Resource {
	return fakeprovider.Resource{
		TypeName: "contract_thing",
		Attrs: map[string]fakeprovider.Attr{
			"label": {Type: tftypes.String, Optional: true},
			"tier":  {Type: tftypes.String, Optional: true, Computed: true, Default: "standard"},
			"zone":  {Type: tftypes.String, Optional: true, Computed: true},
			"id":    {Type: tftypes.String, Computed: true},
		},
		Apply: func(in map[string]string, n int64) (map[string]string, []*tfprotov6.Diagnostic) {
			return map[string]string{
				"id":   fmt.Sprintf("contract-%d", n),
				"zone": fmt.Sprintf("zone-%d", n),
			}, nil
		},
	}
}

func contractServer(t *testing.T) *fakeprovider.Server {
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	return fakeprovider.New(contractResource())
}

func contractPlan(t *testing.T, cfg, proposed map[string]tftypes.Value) map[string]tftypes.Value {
	t.Helper()
	plan, err := contractServer(t).PlanResourceChange(context.Background(), &tfprotov6.PlanResourceChangeRequest{
		TypeName:         "contract_thing",
		Config:           dynamic(t, contractType, cfg),
		ProposedNewState: dynamic(t, contractType, proposed),
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, plan.Diagnostics)
	return decodeState(t, plan.PlannedState, contractType)
}

// unsetCfg is a config that sets only label: every computed attr arrives null,
// as Terraform sends it for an attribute the configuration does not mention.
func unsetCfg() map[string]tftypes.Value {
	return map[string]tftypes.Value{
		"label": tftypes.NewValue(tftypes.String, "L"),
		"tier":  tftypes.NewValue(tftypes.String, nil),
		"zone":  tftypes.NewValue(tftypes.String, nil),
		"id":    tftypes.NewValue(tftypes.String, nil),
	}
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

	planned := contractPlan(t, cfg, proposed)

	if got := str(t, planned["zone"]); got != "eu-central" {
		t.Errorf("zone = %q, want eu-central", got)
	}
}

func TestDeclaredDefaultFiresWhenConfigIsNull(t *testing.T) {
	ctx := context.Background()
	srv := contractServer(t)
	config := dynamic(t, contractType, unsetCfg())

	plan, err := srv.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName: "contract_thing", Config: config, ProposedNewState: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, plan.Diagnostics)
	planned := decodeState(t, plan.PlannedState, contractType)
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
	noErr(t, apply.Diagnostics)
	st := decodeState(t, apply.NewState, contractType)
	if got := str(t, st["tier"]); got != "standard" {
		t.Errorf("tier = %q after apply, want standard: the default did not survive", got)
	}
}

func TestConfiguredValueBeatsTheDefault(t *testing.T) {
	ctx := context.Background()
	srv := contractServer(t)
	cfg := unsetCfg()
	cfg["tier"] = tftypes.NewValue(tftypes.String, "premium")
	config := dynamic(t, contractType, cfg)

	plan, err := srv.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName: "contract_thing", Config: config, ProposedNewState: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, plan.Diagnostics)
	if got := str(t, decodeState(t, plan.PlannedState, contractType)["tier"]); got != "premium" {
		t.Fatalf("tier = %q at plan, want premium", got)
	}

	apply, err := srv.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{
		TypeName: "contract_thing", Config: config, PlannedState: plan.PlannedState,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, apply.Diagnostics)
	if got := str(t, decodeState(t, apply.NewState, contractType)["tier"]); got != "premium" {
		t.Errorf("tier = %q after apply, want premium", got)
	}
}

// A prior value outranks the default: an existing resource is not re-defaulted.
func TestPriorValueBeatsTheDefault(t *testing.T) {
	proposed := unsetCfg()
	proposed["tier"] = tftypes.NewValue(tftypes.String, "legacy")

	planned := contractPlan(t, unsetCfg(), proposed)

	if got := str(t, planned["tier"]); got != "legacy" {
		t.Errorf("tier = %q, want legacy: a prior value must outrank the default", got)
	}
}

// replaceType adds a force-new attr to the contract shape.
var replaceType = objType(map[string]tftypes.Type{
	"image": tftypes.String,
	"label": tftypes.String,
	"zone":  tftypes.String,
	"id":    tftypes.String,
})

func replaceResource() fakeprovider.Resource {
	return fakeprovider.Resource{
		TypeName: "replace_thing",
		Attrs: map[string]fakeprovider.Attr{
			"image": {Type: tftypes.String, Optional: true, ForceNew: true},
			"label": {Type: tftypes.String, Optional: true},
			"zone":  {Type: tftypes.String, Optional: true, Computed: true, ForceNew: true},
			"id":    {Type: tftypes.String, Computed: true},
		},
		Apply: func(_ map[string]string, n int64) (map[string]string, []*tfprotov6.Diagnostic) {
			return map[string]string{
				"id":   fmt.Sprintf("replace-%d", n),
				"zone": fmt.Sprintf("zone-%d", n),
			}, nil
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
		Config:           dynamic(t, replaceType, cfg),
		ProposedNewState: dynamic(t, replaceType, proposedFrom(cfg, prior)),
	}
	if prior != nil {
		req.PriorState = dynamic(t, replaceType, prior)
	}
	resp, err := fakeprovider.New(replaceResource()).PlanResourceChange(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, resp.Diagnostics)
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

// The provider-filled attr must reach prior state holding a value the config
// never set. That is the precondition for the defect in bean nixform2-aur6.
func TestProviderFilledAttrIsUnknownAtPlanAndSetAtApply(t *testing.T) {
	ctx := context.Background()
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")
	srv := fakeprovider.New(replaceResource())
	config := dynamic(t, replaceType, replaceAttrs("ami-old", "L"))

	plan, err := srv.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName: "replace_thing", Config: config, ProposedNewState: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, plan.Diagnostics)
	if decodeState(t, plan.PlannedState, replaceType)["zone"].IsKnown() {
		t.Fatalf("zone known at plan; it has no default and must be unknown")
	}

	apply, err := srv.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{
		TypeName: "replace_thing", Config: config, PlannedState: plan.PlannedState,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, apply.Diagnostics)
	if got := str(t, decodeState(t, apply.NewState, replaceType)["zone"]); got != "zone-0" {
		t.Errorf("zone = %q after apply, want zone-0 chosen by the provider", got)
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
// through a spawned provider. Every fake otherwise succeeds at everything, and
// a failure path that cannot be exercised is one whose behaviour is assumed.
func TestInjectedApplyFailure(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_FAIL", "apply:replace_thing")
	srv := fakeprovider.New(replaceResource())
	config := dynamic(t, replaceType, replaceAttrs("ami-old", "L"))

	// Deterministic: the same request fails identically twice.
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
	srv := fakeprovider.New(replaceResource())
	config := dynamic(t, replaceType, replaceAttrs("ami-old", "L"))

	resp, err := srv.ApplyResourceChange(context.Background(), &tfprotov6.ApplyResourceChangeRequest{
		TypeName: "replace_thing", Config: config, PlannedState: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, resp.Diagnostics)
}

// Injection is addressed by operation AND resource type, so one provider can
// succeed at a destroy and fail the apply that follows it within one run.
func TestInjectionIsAddressedByOperationAndType(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_FAIL", "apply:other_thing")
	srv := fakeprovider.New(replaceResource())
	config := dynamic(t, replaceType, replaceAttrs("ami-old", "L"))

	resp, err := srv.ApplyResourceChange(context.Background(), &tfprotov6.ApplyResourceChangeRequest{
		TypeName: "replace_thing", Config: config, PlannedState: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, resp.Diagnostics)
}

// upgradeResource declares a v2 in which `label` was renamed from `tag`, so an
// upgrade has something real to carry across rather than re-encoding.
func upgradeResource() fakeprovider.Resource {
	return fakeprovider.Resource{
		TypeName: "upgrade_thing",
		Attrs: map[string]fakeprovider.Attr{
			"label": {Type: tftypes.String, Optional: true},
			"id":    {Type: tftypes.String, Computed: true},
		},
		Renames: []fakeprovider.Rename{{AtVersion: 2, From: "tag", To: "label"}},
		Apply: func(_ map[string]string, n int64) (map[string]string, []*tfprotov6.Diagnostic) {
			return map[string]string{"id": fmt.Sprintf("upgrade-%d", n)}, nil
		},
	}
}

var upgradeType = objType(map[string]tftypes.Type{
	"label": tftypes.String,
	"id":    tftypes.String,
})

func TestDeclaredSchemaVersionIsReported(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_SCHEMA_VERSION", "2")
	resp, err := fakeprovider.New(upgradeResource()).GetProviderSchema(
		context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.ResourceSchemas["upgrade_thing"].Version; got != 2 {
		t.Errorf("schema version = %d, want 2", got)
	}
}

// Unset keeps the 1 the fakes have always hardcoded, which is why no existing
// test changes.
func TestSchemaVersionUnsetKeepsOne(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_SCHEMA_VERSION", "")
	resp, err := fakeprovider.New(upgradeResource()).GetProviderSchema(
		context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.ResourceSchemas["upgrade_thing"].Version; got != 1 {
		t.Errorf("schema version = %d, want 1 when unset", got)
	}
}

// The upgrade must TRANSFORM. Re-encoding at the current type, which is what the
// fakes did before, cannot carry a renamed value across.
func TestUpgradeCarriesARenamedAttribute(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_SCHEMA_VERSION", "2")
	resp, err := fakeprovider.New(upgradeResource()).UpgradeResourceState(
		context.Background(), &tfprotov6.UpgradeResourceStateRequest{
			TypeName: "upgrade_thing",
			Version:  1,
			RawState: &tfprotov6.RawState{JSON: []byte(`{"tag":"kept","id":"upgrade-0"}`)},
		})
	if err != nil {
		t.Fatal(err)
	}
	st := decodeState(t, resp.UpgradedState, upgradeType)
	if got := str(t, st["label"]); got != "kept" {
		t.Errorf("label = %q after upgrading from v1, want the value stored under tag", got)
	}
}

// The REQUEST decides which version to upgrade from. A fake that assumed one
// would agree with a caller that sent the wrong version.
func TestUpgradeHonoursTheRequestedVersion(t *testing.T) {
	t.Setenv("TERRAE_NIVIS_FAKE_SCHEMA_VERSION", "2")
	resp, err := fakeprovider.New(upgradeResource()).UpgradeResourceState(
		context.Background(), &tfprotov6.UpgradeResourceStateRequest{
			TypeName: "upgrade_thing",
			Version:  2, // already current: the rename must NOT be applied
			RawState: &tfprotov6.RawState{JSON: []byte(`{"label":"already","id":"upgrade-0"}`)},
		})
	if err != nil {
		t.Fatal(err)
	}
	st := decodeState(t, resp.UpgradedState, upgradeType)
	if got := str(t, st["label"]); got != "already" {
		t.Errorf("label = %q, want it left alone when the state is already current", got)
	}
}
