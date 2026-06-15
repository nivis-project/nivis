package fakeprovider_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wearetechnative/nixform/internal/fakeprovider"
)

// helpers ---------------------------------------------------------------------

func objType(attrs map[string]tftypes.Type) tftypes.Object {
	return tftypes.Object{AttributeTypes: attrs}
}

func dynamic(t *testing.T, typ tftypes.Object, vals map[string]tftypes.Value) *tfprotov6.DynamicValue {
	t.Helper()
	dv, err := tfprotov6.NewDynamicValue(typ, tftypes.NewValue(typ, vals))
	if err != nil {
		t.Fatalf("NewDynamicValue: %v", err)
	}
	return &dv
}

func decodeState(t *testing.T, dv *tfprotov6.DynamicValue, typ tftypes.Object) map[string]tftypes.Value {
	t.Helper()
	v, err := dv.Unmarshal(typ)
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

func noErr(t *testing.T, diags []*tfprotov6.Diagnostic) {
	t.Helper()
	for _, d := range diags {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("unexpected error diagnostic: %s — %s", d.Summary, d.Detail)
		}
	}
}

// alphaType/betaType mirror the resource object shapes.
var alphaType = objType(map[string]tftypes.Type{
	"label": tftypes.String, "id": tftypes.String, "value": tftypes.String,
})
var betaType = objType(map[string]tftypes.Type{
	"from": tftypes.String, "endpoint": tftypes.String,
})

// alphaResource / betaResource mirror the provider definitions so the test is
// self-contained.
func alphaResource() fakeprovider.Resource {
	return fakeprovider.Resource{
		TypeName: "alpha_token",
		Attrs: map[string]fakeprovider.Attr{
			"label": {Type: tftypes.String, Optional: true},
			"id":    {Type: tftypes.String, Computed: true},
			"value": {Type: tftypes.String, Computed: true},
		},
		Apply: func(in map[string]string, n int64) (map[string]string, []*tfprotov6.Diagnostic) {
			return map[string]string{
				"id":    fmt.Sprintf("alpha-%d", n),
				"value": fmt.Sprintf("alpha:%s:%d", in["label"], n),
			}, nil
		},
	}
}

func betaResource() fakeprovider.Resource {
	return fakeprovider.Resource{
		TypeName: "beta_record",
		Attrs: map[string]fakeprovider.Attr{
			"from":     {Type: tftypes.String, Required: true},
			"endpoint": {Type: tftypes.String, Computed: true},
		},
		Apply: func(in map[string]string, _ int64) (map[string]string, []*tfprotov6.Diagnostic) {
			return map[string]string{"endpoint": fmt.Sprintf("beta://%s", in["from"])}, nil
		},
	}
}

// alphaServer / betaServer build a server with the counter explicitly seeded to
// 0 (env cleared) so the seed-0 assertions are hermetic regardless of ambient
// NIXFORM_FAKE_COUNTER. TestCounterSeedFromEnv sets the env itself.
func alphaServer(t *testing.T) *fakeprovider.Server {
	t.Setenv("NIXFORM_FAKE_COUNTER", "")
	return fakeprovider.New(alphaResource())
}

func betaServer(t *testing.T) *fakeprovider.Server {
	t.Setenv("NIXFORM_FAKE_COUNTER", "")
	return fakeprovider.New(betaResource())
}

// tests -----------------------------------------------------------------------

func TestGetProviderSchema(t *testing.T) {
	ctx := context.Background()
	resp, err := alphaServer(t).GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := resp.ResourceSchemas["alpha_token"]; !ok {
		t.Fatalf("schema missing alpha_token; got %v", resp.ResourceSchemas)
	}
}

func TestAlphaPlanApplyRoundTrip(t *testing.T) {
	ctx := context.Background()
	srv := alphaServer(t) // counter seeds at 0 (NIXFORM_FAKE_COUNTER unset)

	cfg := map[string]tftypes.Value{
		"label": tftypes.NewValue(tftypes.String, "rec-X"),
		"id":    tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"value": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	}
	config := dynamic(t, alphaType, cfg)

	// Plan: computed attrs must be unknown.
	plan, err := srv.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName: "alpha_token", Config: config, ProposedNewState: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, plan.Diagnostics)
	planned := decodeState(t, plan.PlannedState, alphaType)
	if planned["id"].IsKnown() || planned["value"].IsKnown() {
		t.Fatalf("expected id/value unknown at plan, got id=%v value=%v", planned["id"], planned["value"])
	}

	// Apply: computed attrs become known and exact.
	apply, err := srv.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{
		TypeName: "alpha_token", Config: config, PlannedState: plan.PlannedState,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, apply.Diagnostics)
	st := decodeState(t, apply.NewState, alphaType)
	for _, k := range []string{"id", "value"} {
		if !st[k].IsKnown() {
			t.Fatalf("attr %q still unknown after apply", k)
		}
	}
	if got := str(t, st["id"]); got != "alpha-0" {
		t.Errorf("id = %q, want alpha-0", got)
	}
	if got := str(t, st["value"]); got != "alpha:rec-X:0" {
		t.Errorf("value = %q, want alpha:rec-X:0", got)
	}
}

func TestAlphaLabelAbsent(t *testing.T) {
	ctx := context.Background()
	srv := alphaServer(t)
	cfg := map[string]tftypes.Value{
		"label": tftypes.NewValue(tftypes.String, nil), // null label (e2e resource A)
		"id":    tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"value": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	}
	config := dynamic(t, alphaType, cfg)
	apply, err := srv.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{
		TypeName: "alpha_token", Config: config, PlannedState: config,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, apply.Diagnostics)
	st := decodeState(t, apply.NewState, alphaType)
	if got := str(t, st["value"]); got != "alpha::0" {
		t.Errorf("value = %q, want alpha::0 (empty label segment)", got)
	}
	if got := str(t, st["id"]); got != "alpha-0" {
		t.Errorf("id = %q, want alpha-0", got)
	}
}

func TestAlphaCounterIncrements(t *testing.T) {
	ctx := context.Background()
	srv := alphaServer(t)
	cfg := map[string]tftypes.Value{
		"label": tftypes.NewValue(tftypes.String, "L"),
		"id":    tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"value": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	}
	config := dynamic(t, alphaType, cfg)
	var ids []string
	for i := 0; i < 3; i++ {
		apply, err := srv.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{
			TypeName: "alpha_token", Config: config, PlannedState: config,
		})
		if err != nil {
			t.Fatal(err)
		}
		st := decodeState(t, apply.NewState, alphaType)
		ids = append(ids, str(t, st["id"]))
	}
	want := []string{"alpha-0", "alpha-1", "alpha-2"}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("apply %d: id=%q want %q", i, ids[i], want[i])
		}
	}
}

func TestCounterSeedFromEnv(t *testing.T) {
	t.Setenv("NIXFORM_FAKE_COUNTER", "5")
	ctx := context.Background()
	srv := fakeprovider.New(alphaResource()) // New() reads the env at construction
	cfg := dynamic(t, alphaType, map[string]tftypes.Value{
		"label": tftypes.NewValue(tftypes.String, "L"),
		"id":    tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"value": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	})
	apply, err := srv.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{
		TypeName: "alpha_token", Config: cfg, PlannedState: cfg,
	})
	if err != nil {
		t.Fatal(err)
	}
	st := decodeState(t, apply.NewState, alphaType)
	if got := str(t, st["id"]); got != "alpha-5" {
		t.Errorf("seeded id = %q, want alpha-5", got)
	}
}

func TestBetaPlanApplyAndRequired(t *testing.T) {
	ctx := context.Background()
	srv := betaServer(t)

	// Required input missing -> diagnostic.
	missing := dynamic(t, betaType, map[string]tftypes.Value{
		"from":     tftypes.NewValue(tftypes.String, nil),
		"endpoint": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	})
	vresp, err := srv.ValidateResourceConfig(ctx, &tfprotov6.ValidateResourceConfigRequest{
		TypeName: "beta_record", Config: missing,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(vresp.Diagnostics) == 0 {
		t.Fatal("expected a required-argument diagnostic for null 'from'")
	}

	// Valid: plan unknown endpoint, apply -> beta://<from>.
	cfg := dynamic(t, betaType, map[string]tftypes.Value{
		"from":     tftypes.NewValue(tftypes.String, "alpha:rec-X:0"),
		"endpoint": tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
	})
	plan, err := srv.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
		TypeName: "beta_record", Config: cfg, ProposedNewState: cfg,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, plan.Diagnostics)
	planned := decodeState(t, plan.PlannedState, betaType)
	if planned["endpoint"].IsKnown() {
		t.Fatal("expected endpoint unknown at plan")
	}
	apply, err := srv.ApplyResourceChange(ctx, &tfprotov6.ApplyResourceChangeRequest{
		TypeName: "beta_record", Config: cfg, PlannedState: plan.PlannedState,
	})
	if err != nil {
		t.Fatal(err)
	}
	noErr(t, apply.Diagnostics)
	st := decodeState(t, apply.NewState, betaType)
	if got := str(t, st["endpoint"]); got != "beta://alpha:rec-X:0" {
		t.Errorf("endpoint = %q, want beta://alpha:rec-X:0", got)
	}
}
