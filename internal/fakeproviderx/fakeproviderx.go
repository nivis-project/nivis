// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package fakeproviderx is a richer-typed tfprotov6 fake provider than the
// string-only internal/fakeprovider: its attributes may be lists, maps, or
// nested objects, so it exercises the value codec end-to-end. Apply funcs take
// and return plain Go values (using internal/tfcodec to bridge to tftypes), so
// they also exercise the codec from the provider side.
package fakeproviderx

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync/atomic"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wearetechnative/nivis/internal/tfcodec"
)

// Attr is a rich-typed resource attribute.
type Attr struct {
	Type     tftypes.Type
	Required bool
	Optional bool
	Computed bool
}

// Resource is a rich fake resource: a type name, its attributes, and a pure
// Apply that computes the Computed attributes from the (decoded) input values
// plus a counter. Inputs/outputs are plain Go values (string, float64, []any,
// map[string]any) — the same shapes the executor's codec produces/consumes.
type Resource struct {
	TypeName string
	Attrs    map[string]Attr
	Apply    func(inputs map[string]interface{}, counter int64) map[string]interface{}
}

func (r Resource) objType() tftypes.Object {
	at := make(map[string]tftypes.Type, len(r.Attrs))
	for n, a := range r.Attrs {
		at[n] = a.Type
	}
	return tftypes.Object{AttributeTypes: at}
}

func (r Resource) schema() *tfprotov6.Schema {
	var sas []*tfprotov6.SchemaAttribute
	for name, a := range r.Attrs {
		sas = append(sas, &tfprotov6.SchemaAttribute{
			Name: name, Type: a.Type,
			Required: a.Required, Optional: a.Optional, Computed: a.Computed,
		})
	}
	return &tfprotov6.Schema{Version: 1, Block: &tfprotov6.SchemaBlock{Version: 1, Attributes: sas}}
}

// Counter is the seedable deterministic counter (TERRAE_NIVIS_FAKE_COUNTER).
type Counter struct{ n int64 }

func NewCounter() *Counter {
	start := int64(0)
	if s := os.Getenv("TERRAE_NIVIS_FAKE_COUNTER"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			start = v
		}
	}
	return &Counter{n: start - 1}
}
func (c *Counter) Next() int64 { return atomic.AddInt64(&c.n, 1) }

// Server serves one rich Resource over tfprotov6.
type Server struct {
	r       Resource
	counter *Counter
}

func New(r Resource) *Server { return &Server{r: r, counter: NewCounter()} }

var _ tfprotov6.ProviderServer = (*Server)(nil)

// --- value helpers ----------------------------------------------------------

func (s *Server) decode(dv *tfprotov6.DynamicValue) (map[string]tftypes.Value, error) {
	if dv == nil {
		return map[string]tftypes.Value{}, nil
	}
	v, err := dv.Unmarshal(s.r.objType())
	if err != nil {
		return nil, err
	}
	if v.IsNull() || !v.IsKnown() {
		return map[string]tftypes.Value{}, nil
	}
	m := map[string]tftypes.Value{}
	if err := v.As(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func (s *Server) encode(vals map[string]tftypes.Value) (*tfprotov6.DynamicValue, error) {
	dv, err := tfprotov6.NewDynamicValue(s.r.objType(), tftypes.NewValue(s.r.objType(), vals))
	if err != nil {
		return nil, err
	}
	return &dv, nil
}

// --- meaningful RPCs --------------------------------------------------------

func (s *Server) GetProviderSchema(context.Context, *tfprotov6.GetProviderSchemaRequest) (*tfprotov6.GetProviderSchemaResponse, error) {
	return &tfprotov6.GetProviderSchemaResponse{
		Provider:        &tfprotov6.Schema{Version: 1, Block: &tfprotov6.SchemaBlock{Version: 1}},
		ResourceSchemas: map[string]*tfprotov6.Schema{s.r.TypeName: s.r.schema()},
	}, nil
}

func (s *Server) GetMetadata(context.Context, *tfprotov6.GetMetadataRequest) (*tfprotov6.GetMetadataResponse, error) {
	return &tfprotov6.GetMetadataResponse{
		ServerCapabilities: &tfprotov6.ServerCapabilities{},
		Resources:          []tfprotov6.ResourceMetadata{{TypeName: s.r.TypeName}},
	}, nil
}

func (s *Server) PlanResourceChange(_ context.Context, req *tfprotov6.PlanResourceChangeRequest) (*tfprotov6.PlanResourceChangeResponse, error) {
	proposed, err := decodeOrNull(req.ProposedNewState, s.r.objType())
	if err != nil {
		return nil, err
	}
	if proposed.IsNull() {
		dv, err := s.encodeValue(tftypes.NewValue(s.r.objType(), nil))
		if err != nil {
			return nil, err
		}
		return &tfprotov6.PlanResourceChangeResponse{PlannedState: dv}, nil
	}
	cfg, err := s.decode(req.Config)
	if err != nil {
		return nil, err
	}
	out := map[string]tftypes.Value{}
	for name, a := range s.r.Attrs {
		if a.Computed && !a.Optional {
			out[name] = tftypes.NewValue(a.Type, tftypes.UnknownValue) // unknown at plan
			continue
		}
		if v, ok := cfg[name]; ok {
			out[name] = v
		} else {
			out[name] = tftypes.NewValue(a.Type, nil)
		}
	}
	dv, err := s.encode(out)
	if err != nil {
		return nil, err
	}
	return &tfprotov6.PlanResourceChangeResponse{PlannedState: dv}, nil
}

func (s *Server) ApplyResourceChange(_ context.Context, req *tfprotov6.ApplyResourceChangeRequest) (*tfprotov6.ApplyResourceChangeResponse, error) {
	planned, err := decodeOrNull(req.PlannedState, s.r.objType())
	if err != nil {
		return nil, err
	}
	if planned.IsNull() {
		dv, err := s.encodeValue(tftypes.NewValue(s.r.objType(), nil))
		if err != nil {
			return nil, err
		}
		return &tfprotov6.ApplyResourceChangeResponse{NewState: dv}, nil
	}
	cfg, err := s.decode(req.Config)
	if err != nil {
		return nil, err
	}
	// Decode inputs to plain Go for the Apply func.
	inputs := map[string]interface{}{}
	for name, a := range s.r.Attrs {
		if a.Computed && !a.Optional {
			continue
		}
		if v, ok := cfg[name]; ok {
			gv, known, err := tfcodec.ValueToGo(v)
			if err != nil {
				return nil, fmt.Errorf("decode input %q: %w", name, err)
			}
			if known && gv != nil {
				inputs[name] = gv
			}
		}
	}
	computed := s.r.Apply(inputs, s.counter.Next())

	out := map[string]tftypes.Value{}
	for name, a := range s.r.Attrs {
		if a.Computed {
			if v, ok := cfg[name]; ok && v.IsKnown() && !v.IsNull() {
				out[name] = v
				continue
			}
			gv, err := tfcodec.GoToValue(a.Type, computed[name])
			if err != nil {
				return nil, fmt.Errorf("encode computed %q: %w", name, err)
			}
			out[name] = gv
			continue
		}
		if v, ok := cfg[name]; ok {
			out[name] = v
		} else {
			out[name] = tftypes.NewValue(a.Type, nil)
		}
	}
	dv, err := s.encode(out)
	if err != nil {
		return nil, err
	}
	return &tfprotov6.ApplyResourceChangeResponse{NewState: dv}, nil
}

func (s *Server) ReadResource(_ context.Context, req *tfprotov6.ReadResourceRequest) (*tfprotov6.ReadResourceResponse, error) {
	return &tfprotov6.ReadResourceResponse{NewState: req.CurrentState}, nil
}

func (s *Server) ValidateResourceConfig(context.Context, *tfprotov6.ValidateResourceConfigRequest) (*tfprotov6.ValidateResourceConfigResponse, error) {
	return &tfprotov6.ValidateResourceConfigResponse{}, nil
}
func (s *Server) ValidateProviderConfig(context.Context, *tfprotov6.ValidateProviderConfigRequest) (*tfprotov6.ValidateProviderConfigResponse, error) {
	return &tfprotov6.ValidateProviderConfigResponse{}, nil
}
func (s *Server) ConfigureProvider(context.Context, *tfprotov6.ConfigureProviderRequest) (*tfprotov6.ConfigureProviderResponse, error) {
	return &tfprotov6.ConfigureProviderResponse{}, nil
}
func (s *Server) StopProvider(context.Context, *tfprotov6.StopProviderRequest) (*tfprotov6.StopProviderResponse, error) {
	return &tfprotov6.StopProviderResponse{}, nil
}
func (s *Server) UpgradeResourceState(_ context.Context, req *tfprotov6.UpgradeResourceStateRequest) (*tfprotov6.UpgradeResourceStateResponse, error) {
	if req.RawState == nil {
		return &tfprotov6.UpgradeResourceStateResponse{}, nil
	}
	v, err := req.RawState.Unmarshal(s.r.objType())
	if err != nil {
		return nil, err
	}
	dv, err := s.encodeValue(v)
	if err != nil {
		return nil, err
	}
	return &tfprotov6.UpgradeResourceStateResponse{UpgradedState: dv}, nil
}

func (s *Server) encodeValue(v tftypes.Value) (*tfprotov6.DynamicValue, error) {
	dv, err := tfprotov6.NewDynamicValue(s.r.objType(), v)
	if err != nil {
		return nil, err
	}
	return &dv, nil
}

func decodeOrNull(dv *tfprotov6.DynamicValue, typ tftypes.Object) (tftypes.Value, error) {
	if dv == nil {
		return tftypes.NewValue(typ, nil), nil
	}
	return dv.Unmarshal(typ)
}

// --- unimplemented RPCs -----------------------------------------------------

func unimpl(rpc string) *tfprotov6.Diagnostic {
	return &tfprotov6.Diagnostic{Severity: tfprotov6.DiagnosticSeverityError, Summary: "Unimplemented",
		Detail: fmt.Sprintf("fakeproviderx does not implement %s", rpc)}
}

func (s *Server) GetResourceIdentitySchemas(context.Context, *tfprotov6.GetResourceIdentitySchemasRequest) (*tfprotov6.GetResourceIdentitySchemasResponse, error) {
	return &tfprotov6.GetResourceIdentitySchemasResponse{}, nil
}
func (s *Server) UpgradeResourceIdentity(context.Context, *tfprotov6.UpgradeResourceIdentityRequest) (*tfprotov6.UpgradeResourceIdentityResponse, error) {
	return &tfprotov6.UpgradeResourceIdentityResponse{Diagnostics: []*tfprotov6.Diagnostic{unimpl("UpgradeResourceIdentity")}}, nil
}
func (s *Server) ImportResourceState(context.Context, *tfprotov6.ImportResourceStateRequest) (*tfprotov6.ImportResourceStateResponse, error) {
	return &tfprotov6.ImportResourceStateResponse{Diagnostics: []*tfprotov6.Diagnostic{unimpl("ImportResourceState")}}, nil
}
func (s *Server) MoveResourceState(context.Context, *tfprotov6.MoveResourceStateRequest) (*tfprotov6.MoveResourceStateResponse, error) {
	return &tfprotov6.MoveResourceStateResponse{Diagnostics: []*tfprotov6.Diagnostic{unimpl("MoveResourceState")}}, nil
}
func (s *Server) GenerateResourceConfig(context.Context, *tfprotov6.GenerateResourceConfigRequest) (*tfprotov6.GenerateResourceConfigResponse, error) {
	return &tfprotov6.GenerateResourceConfigResponse{Diagnostics: []*tfprotov6.Diagnostic{unimpl("GenerateResourceConfig")}}, nil
}
func (s *Server) ValidateDataResourceConfig(context.Context, *tfprotov6.ValidateDataResourceConfigRequest) (*tfprotov6.ValidateDataResourceConfigResponse, error) {
	return &tfprotov6.ValidateDataResourceConfigResponse{Diagnostics: []*tfprotov6.Diagnostic{unimpl("ValidateDataResourceConfig")}}, nil
}
func (s *Server) ReadDataSource(context.Context, *tfprotov6.ReadDataSourceRequest) (*tfprotov6.ReadDataSourceResponse, error) {
	return &tfprotov6.ReadDataSourceResponse{Diagnostics: []*tfprotov6.Diagnostic{unimpl("ReadDataSource")}}, nil
}
func (s *Server) GetFunctions(context.Context, *tfprotov6.GetFunctionsRequest) (*tfprotov6.GetFunctionsResponse, error) {
	return &tfprotov6.GetFunctionsResponse{}, nil
}
func (s *Server) CallFunction(context.Context, *tfprotov6.CallFunctionRequest) (*tfprotov6.CallFunctionResponse, error) {
	return &tfprotov6.CallFunctionResponse{Error: &tfprotov6.FunctionError{Text: "no functions"}}, nil
}
func (s *Server) ValidateEphemeralResourceConfig(context.Context, *tfprotov6.ValidateEphemeralResourceConfigRequest) (*tfprotov6.ValidateEphemeralResourceConfigResponse, error) {
	return &tfprotov6.ValidateEphemeralResourceConfigResponse{Diagnostics: []*tfprotov6.Diagnostic{unimpl("ValidateEphemeralResourceConfig")}}, nil
}
func (s *Server) OpenEphemeralResource(context.Context, *tfprotov6.OpenEphemeralResourceRequest) (*tfprotov6.OpenEphemeralResourceResponse, error) {
	return &tfprotov6.OpenEphemeralResourceResponse{Diagnostics: []*tfprotov6.Diagnostic{unimpl("OpenEphemeralResource")}}, nil
}
func (s *Server) RenewEphemeralResource(context.Context, *tfprotov6.RenewEphemeralResourceRequest) (*tfprotov6.RenewEphemeralResourceResponse, error) {
	return &tfprotov6.RenewEphemeralResourceResponse{Diagnostics: []*tfprotov6.Diagnostic{unimpl("RenewEphemeralResource")}}, nil
}
func (s *Server) CloseEphemeralResource(context.Context, *tfprotov6.CloseEphemeralResourceRequest) (*tfprotov6.CloseEphemeralResourceResponse, error) {
	return &tfprotov6.CloseEphemeralResourceResponse{Diagnostics: []*tfprotov6.Diagnostic{unimpl("CloseEphemeralResource")}}, nil
}
