// Copyright 2026 WeareTechnative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package v5 implements provider.Client over the tfprotov5 wire protocol.
package v5

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wearetechnative/nivis/internal/provider"
	"github.com/wearetechnative/nivis/internal/tfplugin5"
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

// Configure builds the provider config object from the (cached) provider schema
// block, encodes the given config (absent attrs -> null), and calls the v5
// Configure RPC. Empty config against a config-free provider is a no-op.
func (b *Backend) Configure(ctx context.Context, config map[string]interface{}) error {
	resp, err := b.schema(ctx)
	if err != nil {
		return err
	}
	objType, err := objectType(resp.GetProvider().GetBlock())
	if err != nil {
		return fmt.Errorf("provider config schema: %w", err)
	}
	cfgDV, err := encodeConfig(objType, map[string]bool{}, config)
	if err != nil {
		return fmt.Errorf("encode provider config: %w", err)
	}
	confResp, err := b.client.Configure(ctx, &tfplugin5.Configure_Request{
		TerraformVersion: "1.0.0",
		Config:           cfgDV,
	})
	if err != nil {
		return fmt.Errorf("Configure: %w", err)
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
	// PriorState is the stored attributes for an existing resource, or a proper
	// NULL value of the resource type for a create. (It must be a real null, not
	// an empty DynamicValue — SDKv2 providers panic decoding empty msgpack.)
	prior, err := priorState(raw.objType, req.Prior)
	if err != nil {
		return provider.PlanResult{}, err
	}
	resp, err := b.client.PlanResourceChange(ctx, &tfplugin5.PlanResourceChange_Request{
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
	unknown, err := unknownAttrs(raw.objType, resp.GetPlannedState())
	if err != nil {
		return provider.PlanResult{}, err
	}
	// A replace is only meaningful when there was a prior resource; for a create
	// the provider may still report paths, which we ignore.
	requiresReplace := req.Prior != nil && len(resp.GetRequiresReplace()) > 0
	// No-op: prior state exists, nothing is unknown-after-apply, no replace, and
	// the planned state decodes equal to the prior state — nothing to do.
	// No-op when there is prior state, no replacement, and every attribute whose
	// planned value is KNOWN equals the prior value. Computed attributes the
	// provider re-marks unknown-after-apply on a re-plan (arn, etag, …) are
	// ignored: for an unchanged resource the provider keeps the prior value for
	// them, so an apply would be a churn-free no change. (We can't gate on
	// len(unknown)==0 — real resources always have computed attrs.)
	noop := false
	if req.Prior != nil && !requiresReplace {
		planned, derr := decodeState(raw.objType, resp.GetPlannedState())
		if derr == nil {
			noop = knownAttrsMatchPrior(planned, req.Prior, unknown)
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
	cfgDV, err := encodeConfig(raw.objType, raw.computed, req.ResolvedCfg)
	if err != nil {
		return provider.ApplyResult{}, fmt.Errorf("encode config: %w", err)
	}
	planned, _ := req.PlannedState.(*tfplugin5.DynamicValue)
	// PriorState is the stored attributes for an in-place update, or null for a
	// create. (On a replace the executor destroys the prior resource first and
	// applies the new one as a create, so Prior is nil here.)
	prior, err := priorState(raw.objType, req.Prior)
	if err != nil {
		return provider.ApplyResult{}, err
	}
	resp, err := b.client.ApplyResourceChange(ctx, &tfplugin5.ApplyResourceChange_Request{
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
