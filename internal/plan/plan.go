// Package plan runs the provider PlanResourceChange RPC for a ready resource and
// renders a human-readable plan. It has no side effects.
package plan

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wearetechnative/nixform/internal/ir"
	"github.com/wearetechnative/nixform/internal/tfplugin6"
	"github.com/wearetechnative/nixform/internal/tfvalue"
)

// ResourceSchema bundles a resource type's object type and which attributes are
// computed, derived once from the provider schema and reused by plan and apply.
type ResourceSchema struct {
	ObjType  tftypes.Object
	Computed map[string]bool
}

// SchemaFor fetches the provider schema and extracts the schema for one resource
// type, returning the parsed object type and computed-attr set.
func SchemaFor(ctx context.Context, client tfplugin6.ProviderClient, resourceType string) (ResourceSchema, error) {
	resp, err := client.GetProviderSchema(ctx, &tfplugin6.GetProviderSchema_Request{})
	if err != nil {
		return ResourceSchema{}, fmt.Errorf("GetProviderSchema: %w", err)
	}
	if err := diagErr(resp.GetDiagnostics()); err != nil {
		return ResourceSchema{}, err
	}
	sch, ok := resp.GetResourceSchemas()[resourceType]
	if !ok {
		return ResourceSchema{}, fmt.Errorf("provider has no resource type %q", resourceType)
	}
	objType, err := tfvalue.ObjectType(sch.GetBlock())
	if err != nil {
		return ResourceSchema{}, err
	}
	computed := map[string]bool{}
	for _, a := range sch.GetBlock().GetAttributes() {
		if a.GetComputed() {
			computed[a.GetName()] = true
		}
	}
	return ResourceSchema{ObjType: objType, Computed: computed}, nil
}

// Result is the outcome of planning one resource.
type Result struct {
	PlannedState *tfplugin6.DynamicValue
	// UnknownAfterApply lists attributes that are unknown in the planned state
	// (computed values known only after apply).
	UnknownAfterApply []string
	Human             string
}

// Plan calls PlanResourceChange for a ready resource with its resolved config
// (unresolved refs encoded as unknown). It returns the planned state for apply
// plus a human-readable summary.
func Plan(ctx context.Context, client tfplugin6.ProviderClient, rs ResourceSchema, node *ir.ResourceNode, resolvedCfg map[string]interface{}) (Result, error) {
	cfgDV, err := tfvalue.EncodeConfig(rs.ObjType, rs.Computed, resolvedCfg)
	if err != nil {
		return Result{}, fmt.Errorf("encode config: %w", err)
	}
	resp, err := client.PlanResourceChange(ctx, &tfplugin6.PlanResourceChange_Request{
		TypeName:         node.Resource.Type,
		Config:           cfgDV,
		ProposedNewState: cfgDV,
		PriorState:       &tfplugin6.DynamicValue{Msgpack: nil},
	})
	if err != nil {
		return Result{}, fmt.Errorf("PlanResourceChange: %w", err)
	}
	if err := diagErr(resp.GetDiagnostics()); err != nil {
		return Result{}, err
	}

	unknown, err := unknownAttrs(rs.ObjType, resp.GetPlannedState())
	if err != nil {
		return Result{}, err
	}
	return Result{
		PlannedState:      resp.GetPlannedState(),
		UnknownAfterApply: unknown,
		Human:             renderPlan(node.Resource.ID, unknown),
	}, nil
}

// unknownAttrs decodes the planned state and lists attributes whose value is
// unknown (known only after apply).
func unknownAttrs(objType tftypes.Object, dv *tfplugin6.DynamicValue) ([]string, error) {
	if dv == nil || len(dv.GetMsgpack()) == 0 {
		return nil, nil
	}
	v, err := tftypes.ValueFromMsgPack(dv.GetMsgpack(), objType)
	if err != nil {
		return nil, fmt.Errorf("decode planned state: %w", err)
	}
	m := map[string]tftypes.Value{}
	if err := v.As(&m); err != nil {
		return nil, err
	}
	var out []string
	for name, av := range m {
		if !av.IsKnown() {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

func renderPlan(id string, unknown []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s will be created", id)
	if len(unknown) > 0 {
		fmt.Fprintf(&b, " (%s known after apply)", strings.Join(unknown, ", "))
	}
	return b.String()
}

// diagErr converts error-severity diagnostics into a Go error.
func diagErr(diags []*tfplugin6.Diagnostic) error {
	var msgs []string
	for _, d := range diags {
		if d.GetSeverity() == tfplugin6.Diagnostic_ERROR {
			msgs = append(msgs, strings.TrimSpace(d.GetSummary()+": "+d.GetDetail()))
		}
	}
	if len(msgs) > 0 {
		return fmt.Errorf("provider diagnostics: %s", strings.Join(msgs, "; "))
	}
	return nil
}
