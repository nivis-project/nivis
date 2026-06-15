// Package v5 implements provider.Client over the tfprotov5 wire protocol.
package v5

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wearetechnative/nixform/internal/provider"
	"github.com/wearetechnative/nixform/internal/tfplugin5"
)

// Backend wraps a tfprotov5 gRPC client as a version-neutral provider.Client.
type Backend struct {
	client tfplugin5.ProviderClient

	// Provider schema is fetched once and cached (see the v6 backend for why).
	schemaOnce sync.Once
	schemaResp *tfplugin5.GetProviderSchema_Response
	schemaErr  error
}

// New wraps a tfplugin5.ProviderClient.
func New(client tfplugin5.ProviderClient) *Backend { return &Backend{client: client} }

var _ provider.Client = (*Backend)(nil)

type schemaRaw struct {
	objType  tftypes.Object
	computed map[string]bool
}

// schema returns the provider schema, fetching it at most once per backend.
func (b *Backend) schema(ctx context.Context) (*tfplugin5.GetProviderSchema_Response, error) {
	b.schemaOnce.Do(func() {
		resp, err := b.client.GetSchema(ctx, &tfplugin5.GetProviderSchema_Request{})
		if err != nil {
			b.schemaErr = fmt.Errorf("GetSchema: %w", err)
			return
		}
		if err := provider.DiagError(normDiags(resp.GetDiagnostics())); err != nil {
			b.schemaErr = err
			return
		}
		b.schemaResp = resp
	})
	return b.schemaResp, b.schemaErr
}

func (b *Backend) ListResourceTypes(ctx context.Context) ([]string, error) {
	resp, err := b.schema(ctx)
	if err != nil {
		return nil, err
	}
	var types []string
	for t := range resp.GetResourceSchemas() {
		types = append(types, t)
	}
	sort.Strings(types)
	return types, nil
}

func (b *Backend) GetSchema(ctx context.Context, resourceType string) (provider.ResourceSchema, error) {
	resp, err := b.schema(ctx)
	if err != nil {
		return provider.ResourceSchema{}, err
	}
	sch, ok := resp.GetResourceSchemas()[resourceType]
	if !ok {
		return provider.ResourceSchema{}, fmt.Errorf("provider has no resource type %q", resourceType)
	}
	objType, err := objectType(sch.GetBlock())
	if err != nil {
		return provider.ResourceSchema{}, err
	}
	var attrs []provider.Attr
	computed := map[string]bool{}
	for _, a := range sch.GetBlock().GetAttributes() {
		attrs = append(attrs, provider.Attr{
			Name:      a.GetName(),
			TypeKind:  typeKind(a),
			Required:  a.GetRequired(),
			Optional:  a.GetOptional(),
			Computed:  a.GetComputed(),
			Sensitive: a.GetSensitive(),
		})
		if a.GetComputed() {
			computed[a.GetName()] = true
		}
	}
	return provider.ResourceSchema{
		TypeName: resourceType,
		Attrs:    attrs,
		Raw:      schemaRaw{objType: objType, computed: computed},
	}, nil
}

func (b *Backend) Plan(ctx context.Context, req provider.PlanRequest) (provider.PlanResult, error) {
	raw := req.Schema.Raw.(schemaRaw)
	cfgDV, err := encodeConfig(raw.objType, raw.computed, req.ResolvedCfg)
	if err != nil {
		return provider.PlanResult{}, fmt.Errorf("encode config: %w", err)
	}
	resp, err := b.client.PlanResourceChange(ctx, &tfplugin5.PlanResourceChange_Request{
		TypeName:         req.TypeName,
		Config:           cfgDV,
		ProposedNewState: cfgDV,
		PriorState:       &tfplugin5.DynamicValue{},
	})
	if err != nil {
		return provider.PlanResult{}, fmt.Errorf("PlanResourceChange: %w", err)
	}
	diags := normDiags(resp.GetDiagnostics())
	if err := provider.DiagError(diags); err != nil {
		return provider.PlanResult{}, err
	}
	unknown, err := unknownAttrs(raw.objType, resp.GetPlannedState())
	if err != nil {
		return provider.PlanResult{}, err
	}
	return provider.PlanResult{PlannedState: resp.GetPlannedState(), UnknownAfterApply: unknown, Diagnostics: diags}, nil
}

func (b *Backend) Apply(ctx context.Context, req provider.ApplyRequest) (provider.ApplyResult, error) {
	raw := req.Schema.Raw.(schemaRaw)
	cfgDV, err := encodeConfig(raw.objType, raw.computed, req.ResolvedCfg)
	if err != nil {
		return provider.ApplyResult{}, fmt.Errorf("encode config: %w", err)
	}
	planned, _ := req.PlannedState.(*tfplugin5.DynamicValue)
	resp, err := b.client.ApplyResourceChange(ctx, &tfplugin5.ApplyResourceChange_Request{
		TypeName:     req.TypeName,
		Config:       cfgDV,
		PlannedState: planned,
		PriorState:   &tfplugin5.DynamicValue{},
	})
	if err != nil {
		return provider.ApplyResult{}, fmt.Errorf("ApplyResourceChange: %w", err)
	}
	diags := normDiags(resp.GetDiagnostics())
	if err := provider.DiagError(diags); err != nil {
		return provider.ApplyResult{}, err
	}
	attrs, err := decodeState(raw.objType, resp.GetNewState())
	if err != nil {
		return provider.ApplyResult{}, fmt.Errorf("decode new state: %w", err)
	}
	return provider.ApplyResult{Attrs: attrs, Diagnostics: diags}, nil
}

func (b *Backend) Read(ctx context.Context, req provider.ReadRequest) (provider.ReadResult, error) {
	raw := req.Schema.Raw.(schemaRaw)
	cur, err := encodeState(raw.objType, req.Stored)
	if err != nil {
		return provider.ReadResult{}, fmt.Errorf("encode current state: %w", err)
	}
	resp, err := b.client.ReadResource(ctx, &tfplugin5.ReadResource_Request{
		TypeName:     req.TypeName,
		CurrentState: cur,
	})
	if err != nil {
		return provider.ReadResult{}, fmt.Errorf("ReadResource: %w", err)
	}
	diags := normDiags(resp.GetDiagnostics())
	if err := provider.DiagError(diags); err != nil {
		return provider.ReadResult{}, err
	}
	attrs, err := decodeState(raw.objType, resp.GetNewState())
	if err != nil {
		return provider.ReadResult{}, err
	}
	return provider.ReadResult{Attrs: attrs, Diagnostics: diags}, nil
}

func (b *Backend) Destroy(ctx context.Context, req provider.DestroyRequest) (provider.DestroyResult, error) {
	raw := req.Schema.Raw.(schemaRaw)
	prior, err := encodeState(raw.objType, req.Stored)
	if err != nil {
		return provider.DestroyResult{}, fmt.Errorf("encode prior state: %w", err)
	}
	nullPlanned, err := nullState(raw.objType)
	if err != nil {
		return provider.DestroyResult{}, err
	}
	resp, err := b.client.ApplyResourceChange(ctx, &tfplugin5.ApplyResourceChange_Request{
		TypeName:     req.TypeName,
		PriorState:   prior,
		PlannedState: nullPlanned,
		Config:       nullPlanned,
	})
	if err != nil {
		return provider.DestroyResult{}, fmt.Errorf("ApplyResourceChange(delete): %w", err)
	}
	diags := normDiags(resp.GetDiagnostics())
	if err := provider.DiagError(diags); err != nil {
		return provider.DestroyResult{}, err
	}
	return provider.DestroyResult{Diagnostics: diags}, nil
}

func typeKind(a *tfplugin5.Schema_Attribute) string {
	t, err := tftypes.ParseJSONType(a.GetType())
	if err != nil {
		return "dynamic"
	}
	switch {
	case t.Is(tftypes.String):
		return "string"
	case t.Is(tftypes.Number):
		return "number"
	case t.Is(tftypes.Bool):
		return "bool"
	}
	switch t.(type) {
	case tftypes.List:
		return "list"
	case tftypes.Set:
		return "set"
	case tftypes.Map:
		return "map"
	case tftypes.Object:
		return "object"
	}
	return "dynamic"
}

func normDiags(in []*tfplugin5.Diagnostic) []provider.Diagnostic {
	var out []provider.Diagnostic
	for _, d := range in {
		sev := provider.SeverityWarning
		if d.GetSeverity() == tfplugin5.Diagnostic_ERROR {
			sev = provider.SeverityError
		}
		out = append(out, provider.Diagnostic{Severity: sev, Summary: d.GetSummary(), Detail: d.GetDetail()})
	}
	return out
}
