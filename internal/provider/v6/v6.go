// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package v6 implements provider.Client over the tfprotov6 wire protocol.
package v6

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wearetechnative/terrae-nivis/internal/provider"
	"github.com/wearetechnative/terrae-nivis/internal/tfplugin6"
	"github.com/wearetechnative/terrae-nivis/internal/tfvalue"
)

// Backend wraps a tfprotov6 gRPC client as a version-neutral provider.Client.
type Backend struct {
	client tfplugin6.ProviderClient

	// The provider schema is fetched once and cached: GetProviderSchema is
	// expensive (multi-MB for large providers like AWS) and was previously
	// re-issued per resource type, which is O(resources) round trips.
	schemaOnce sync.Once
	schemaResp *tfplugin6.GetProviderSchema_Response
	schemaErr  error
}

// New wraps a tfplugin6.ProviderClient.
func New(client tfplugin6.ProviderClient) *Backend { return &Backend{client: client} }

var _ provider.Client = (*Backend)(nil)

// schemaRaw is the backend-specific handle carried in ResourceSchema.Raw: the
// resource's object type and which attrs are computed (for unknown-at-plan).
type schemaRaw struct {
	objType  tftypes.Object
	computed map[string]bool
}

// schema returns the provider schema, fetching it at most once per backend.
func (b *Backend) schema(ctx context.Context) (*tfplugin6.GetProviderSchema_Response, error) {
	b.schemaOnce.Do(func() {
		resp, err := b.client.GetProviderSchema(ctx, &tfplugin6.GetProviderSchema_Request{})
		if err != nil {
			b.schemaErr = fmt.Errorf("GetProviderSchema: %w", err)
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

// Configure builds the provider config object from the (cached) provider schema
// block, encodes the given config (absent attrs -> null), and calls
// ConfigureProvider. An empty config against a provider with no required config
// is a valid no-op.
func (b *Backend) Configure(ctx context.Context, config map[string]interface{}) error {
	resp, err := b.schema(ctx)
	if err != nil {
		return err
	}
	block := resp.GetProvider().GetBlock()
	objType, err := tfvalue.ObjectType(block)
	if err != nil {
		return fmt.Errorf("provider config schema: %w", err)
	}
	// No computed provider-config attrs at configure time; all attrs are inputs.
	cfgDV, err := tfvalue.EncodeConfig(objType, map[string]bool{}, config)
	if err != nil {
		return fmt.Errorf("encode provider config: %w", err)
	}
	confResp, err := b.client.ConfigureProvider(ctx, &tfplugin6.ConfigureProvider_Request{
		TerraformVersion: "1.0.0",
		Config:           cfgDV,
	})
	if err != nil {
		return fmt.Errorf("ConfigureProvider: %w", err)
	}
	return provider.DiagError(normDiags(confResp.GetDiagnostics()))
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
	objType, err := tfvalue.ObjectType(sch.GetBlock())
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
	cfgDV, err := tfvalue.EncodeConfig(raw.objType, raw.computed, req.ResolvedCfg)
	if err != nil {
		return provider.PlanResult{}, fmt.Errorf("encode config: %w", err)
	}
	// PriorState for a create must be a properly-encoded NULL value of the
	// resource object type, not an empty DynamicValue — real (SDKv2) providers
	// panic trying to decode empty msgpack.
	priorNull, err := tfvalue.NullState(raw.objType)
	if err != nil {
		return provider.PlanResult{}, err
	}
	resp, err := b.client.PlanResourceChange(ctx, &tfplugin6.PlanResourceChange_Request{
		TypeName:         req.TypeName,
		Config:           cfgDV,
		ProposedNewState: cfgDV,
		PriorState:       priorNull,
	})
	if err != nil {
		return provider.PlanResult{}, fmt.Errorf("PlanResourceChange: %w", err)
	}
	diags := normDiags(resp.GetDiagnostics())
	if err := provider.DiagError(diags); err != nil {
		return provider.PlanResult{}, err
	}
	unknown, err := tfvalue.UnknownAttrs(raw.objType, resp.GetPlannedState())
	if err != nil {
		return provider.PlanResult{}, err
	}
	return provider.PlanResult{
		PlannedState:      resp.GetPlannedState(),
		UnknownAfterApply: unknown,
		Diagnostics:       diags,
	}, nil
}

func (b *Backend) Apply(ctx context.Context, req provider.ApplyRequest) (provider.ApplyResult, error) {
	raw := req.Schema.Raw.(schemaRaw)
	cfgDV, err := tfvalue.EncodeConfig(raw.objType, raw.computed, req.ResolvedCfg)
	if err != nil {
		return provider.ApplyResult{}, fmt.Errorf("encode config: %w", err)
	}
	planned, _ := req.PlannedState.(*tfplugin6.DynamicValue)
	priorNull, err := tfvalue.NullState(raw.objType)
	if err != nil {
		return provider.ApplyResult{}, err
	}
	resp, err := b.client.ApplyResourceChange(ctx, &tfplugin6.ApplyResourceChange_Request{
		TypeName:     req.TypeName,
		Config:       cfgDV,
		PlannedState: planned,
		PriorState:   priorNull,
	})
	if err != nil {
		return provider.ApplyResult{}, fmt.Errorf("ApplyResourceChange: %w", err)
	}
	diags := normDiags(resp.GetDiagnostics())
	if err := provider.DiagError(diags); err != nil {
		return provider.ApplyResult{}, err
	}
	attrs, err := tfvalue.DecodeState(raw.objType, resp.GetNewState())
	if err != nil {
		return provider.ApplyResult{}, fmt.Errorf("decode new state: %w", err)
	}
	return provider.ApplyResult{Attrs: attrs, Diagnostics: diags}, nil
}

func (b *Backend) Read(ctx context.Context, req provider.ReadRequest) (provider.ReadResult, error) {
	raw := req.Schema.Raw.(schemaRaw)
	cur, err := tfvalue.EncodeState(raw.objType, req.Stored)
	if err != nil {
		return provider.ReadResult{}, fmt.Errorf("encode current state: %w", err)
	}
	resp, err := b.client.ReadResource(ctx, &tfplugin6.ReadResource_Request{
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
	attrs, err := tfvalue.DecodeState(raw.objType, resp.GetNewState())
	if err != nil {
		return provider.ReadResult{}, err
	}
	return provider.ReadResult{Attrs: attrs, Diagnostics: diags}, nil
}

func (b *Backend) Destroy(ctx context.Context, req provider.DestroyRequest) (provider.DestroyResult, error) {
	raw := req.Schema.Raw.(schemaRaw)
	prior, err := tfvalue.EncodeState(raw.objType, req.Stored)
	if err != nil {
		return provider.DestroyResult{}, fmt.Errorf("encode prior state: %w", err)
	}
	nullPlanned, err := tfvalue.NullState(raw.objType)
	if err != nil {
		return provider.DestroyResult{}, err
	}
	resp, err := b.client.ApplyResourceChange(ctx, &tfplugin6.ApplyResourceChange_Request{
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

// typeKind returns a coarse type-kind string for a schema attribute. A nested
// type (block) is an object; otherwise parse the JSON tftype.
func typeKind(a *tfplugin6.Schema_Attribute) string {
	if a.GetNestedType() != nil {
		return "object"
	}
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

func normDiags(in []*tfplugin6.Diagnostic) []provider.Diagnostic {
	var out []provider.Diagnostic
	for _, d := range in {
		sev := provider.SeverityWarning
		if d.GetSeverity() == tfplugin6.Diagnostic_ERROR {
			sev = provider.SeverityError
		}
		out = append(out, provider.Diagnostic{Severity: sev, Summary: d.GetSummary(), Detail: d.GetDetail()})
	}
	return out
}
