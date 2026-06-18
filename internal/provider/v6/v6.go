// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package v6 implements provider.Client over the tfprotov6 wire protocol.
package v6

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wearetechnative/nivis/internal/provider"
	"github.com/wearetechnative/nivis/internal/tfcodec"
	"github.com/wearetechnative/nivis/internal/tfplugin6"
	"github.com/wearetechnative/nivis/internal/tfvalue"
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
		Blocks:   nestedBlocks(sch.GetBlock()),
		Raw:      schemaRaw{objType: objType, computed: computed},
	}, nil
}

// blockAttrs maps a schema block's flat attributes to provider.Attr.
func blockAttrs(block *tfplugin6.Schema_Block) []provider.Attr {
	var attrs []provider.Attr
	for _, a := range block.GetAttributes() {
		attrs = append(attrs, provider.Attr{
			Name:      a.GetName(),
			TypeKind:  typeKind(a),
			Required:  a.GetRequired(),
			Optional:  a.GetOptional(),
			Computed:  a.GetComputed(),
			Sensitive: a.GetSensitive(),
		})
	}
	return attrs
}

// nestedBlocks maps a schema block's nested blocks (recursively) to the
// version-neutral provider.NestedBlock, so codegen learns each block's nesting
// mode without parsing protobuf.
func nestedBlocks(block *tfplugin6.Schema_Block) []provider.NestedBlock {
	var out []provider.NestedBlock
	for _, b := range block.GetBlockTypes() {
		var nesting provider.BlockNesting
		switch b.GetNesting() {
		case tfplugin6.Schema_NestedBlock_LIST, tfplugin6.Schema_NestedBlock_GROUP:
			nesting = provider.BlockList
		case tfplugin6.Schema_NestedBlock_SET:
			nesting = provider.BlockSet
		case tfplugin6.Schema_NestedBlock_MAP:
			nesting = provider.BlockMap
		default: // SINGLE, INVALID, or unknown: treat as a single attrset
			nesting = provider.BlockSingle
		}
		out = append(out, provider.NestedBlock{
			Name:    b.GetTypeName(),
			Nesting: nesting,
			Attrs:   blockAttrs(b.GetBlock()),
			Blocks:  nestedBlocks(b.GetBlock()),
		})
	}
	return out
}

func (b *Backend) GetDataSourceSchema(ctx context.Context, dataSourceType string) (provider.ResourceSchema, error) {
	resp, err := b.schema(ctx)
	if err != nil {
		return provider.ResourceSchema{}, err
	}
	sch, ok := resp.GetDataSourceSchemas()[dataSourceType]
	if !ok {
		return provider.ResourceSchema{}, fmt.Errorf("provider has no datasource type %q", dataSourceType)
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
		TypeName: dataSourceType,
		Attrs:    attrs,
		Raw:      schemaRaw{objType: objType, computed: computed},
	}, nil
}

func (b *Backend) ReadDataSource(ctx context.Context, req provider.ReadDataSourceRequest) (provider.ReadDataSourceResult, error) {
	raw := req.Schema.Raw.(schemaRaw)
	cfgDV, err := tfvalue.EncodeConfig(raw.objType, raw.computed, req.ResolvedCfg)
	if err != nil {
		return provider.ReadDataSourceResult{}, fmt.Errorf("encode config: %w", err)
	}
	resp, err := b.client.ReadDataSource(ctx, &tfplugin6.ReadDataSource_Request{
		TypeName: req.TypeName,
		Config:   cfgDV,
	})
	if err != nil {
		return provider.ReadDataSourceResult{}, fmt.Errorf("ReadDataSource: %w", err)
	}
	diags := normDiags(resp.GetDiagnostics())
	if err := provider.DiagError(diags); err != nil {
		return provider.ReadDataSourceResult{}, err
	}
	attrs, err := tfvalue.DecodeState(raw.objType, resp.GetState())
	if err != nil {
		return provider.ReadDataSourceResult{}, fmt.Errorf("decode datasource state: %w", err)
	}
	return provider.ReadDataSourceResult{Attrs: attrs, Diagnostics: diags}, nil
}

func (b *Backend) Plan(ctx context.Context, req provider.PlanRequest) (provider.PlanResult, error) {
	raw := req.Schema.Raw.(schemaRaw)
	cfgDV, err := tfvalue.EncodeConfig(raw.objType, raw.computed, req.ResolvedCfg)
	if err != nil {
		return provider.PlanResult{}, fmt.Errorf("encode config: %w", err)
	}
	// PriorState is the stored attributes for an existing resource, or a properly-
	// encoded NULL value for a create (not an empty DynamicValue — real SDKv2
	// providers panic decoding empty msgpack).
	prior, err := priorState6(raw.objType, req.Prior)
	if err != nil {
		return provider.PlanResult{}, err
	}
	resp, err := b.client.PlanResourceChange(ctx, &tfplugin6.PlanResourceChange_Request{
		TypeName:         req.TypeName,
		Config:           cfgDV,
		ProposedNewState: cfgDV,
		PriorState:       prior,
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
	requiresReplace := req.Prior != nil && len(resp.GetRequiresReplace()) > 0
	// No-op: prior exists, no replace, and every KNOWN planned attr equals prior
	// (computed attrs re-marked unknown-after-apply are ignored — see v5).
	noop := false
	if req.Prior != nil && !requiresReplace {
		planned, derr := tfvalue.DecodeState(raw.objType, resp.GetPlannedState())
		if derr == nil {
			noop = tfcodec.KnownAttrsMatchPrior(planned, req.Prior, unknown)
		}
	}
	return provider.PlanResult{
		PlannedState:      resp.GetPlannedState(),
		UnknownAfterApply: unknown,
		RequiresReplace:   requiresReplace,
		NoOp:              noop,
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
	// PriorState is the stored attributes for an in-place update, or null for a
	// create (and for the create half of a replace).
	prior, err := priorState6(raw.objType, req.Prior)
	if err != nil {
		return provider.ApplyResult{}, err
	}
	resp, err := b.client.ApplyResourceChange(ctx, &tfplugin6.ApplyResourceChange_Request{
		TypeName:     req.TypeName,
		Config:       cfgDV,
		PlannedState: planned,
		PriorState:   prior,
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

// priorState6 encodes a resource's stored attributes as PriorState, or a null
// state when there is no prior (a create). nil/empty prior => null.
func priorState6(objType tftypes.Object, prior map[string]interface{}) (*tfplugin6.DynamicValue, error) {
	if len(prior) == 0 {
		return tfvalue.NullState(objType)
	}
	return tfvalue.EncodeState(objType, prior)
}
