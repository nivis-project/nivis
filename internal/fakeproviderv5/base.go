// Copyright 2026 WeareTechnative B.V. and the nixform authors
// SPDX-License-Identifier: Apache-2.0

package fakeproviderv5

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Server is a tfprotov5.ProviderServer for a fake provider. It serves a fixed
// set of Resources and returns unimplemented diagnostics for the RPCs a fake
// does not need (data sources, functions, ephemeral resources, import, move).
type Server struct {
	resources map[string]Resource
	counter   *Counter
}

// New builds a Server from a list of Resources, seeding the deterministic
// counter from NIXFORM_FAKE_COUNTER.
func New(resources ...Resource) *Server {
	m := make(map[string]Resource, len(resources))
	for _, r := range resources {
		m[r.TypeName] = r
	}
	return &Server{resources: m, counter: NewCounter()}
}

var _ tfprotov5.ProviderServer = (*Server)(nil)

func unimplemented(rpc string) *tfprotov5.Diagnostic {
	return &tfprotov5.Diagnostic{
		Severity: tfprotov5.DiagnosticSeverityError,
		Summary:  "Unimplemented",
		Detail:   fmt.Sprintf("This fake provider does not implement %s.", rpc),
	}
}

func (s *Server) lookup(typeName string) (Resource, *tfprotov5.Diagnostic) {
	r, ok := s.resources[typeName]
	if !ok {
		return Resource{}, errDiag("Unknown resource type",
			fmt.Sprintf("This provider has no resource type %q.", typeName))
	}
	return r, nil
}

// --- meaningful RPCs --------------------------------------------------------

func (s *Server) GetMetadata(_ context.Context, _ *tfprotov5.GetMetadataRequest) (*tfprotov5.GetMetadataResponse, error) {
	resp := &tfprotov5.GetMetadataResponse{ServerCapabilities: &tfprotov5.ServerCapabilities{}}
	for name := range s.resources {
		resp.Resources = append(resp.Resources, tfprotov5.ResourceMetadata{TypeName: name})
	}
	return resp, nil
}

func (s *Server) GetProviderSchema(_ context.Context, _ *tfprotov5.GetProviderSchemaRequest) (*tfprotov5.GetProviderSchemaResponse, error) {
	schemas := map[string]*tfprotov5.Schema{}
	for name, r := range s.resources {
		schemas[name] = r.schema()
	}
	return &tfprotov5.GetProviderSchemaResponse{
		Provider:        &tfprotov5.Schema{Version: 1, Block: &tfprotov5.SchemaBlock{Version: 1}},
		ResourceSchemas: schemas,
	}, nil
}

func (s *Server) PrepareProviderConfig(_ context.Context, req *tfprotov5.PrepareProviderConfigRequest) (*tfprotov5.PrepareProviderConfigResponse, error) {
	// Echo the config back unmodified; the fake provider needs no preparation.
	return &tfprotov5.PrepareProviderConfigResponse{PreparedConfig: req.Config}, nil
}

func (s *Server) ConfigureProvider(_ context.Context, _ *tfprotov5.ConfigureProviderRequest) (*tfprotov5.ConfigureProviderResponse, error) {
	return &tfprotov5.ConfigureProviderResponse{}, nil
}

func (s *Server) StopProvider(_ context.Context, _ *tfprotov5.StopProviderRequest) (*tfprotov5.StopProviderResponse, error) {
	return &tfprotov5.StopProviderResponse{}, nil
}

func (s *Server) ValidateResourceTypeConfig(_ context.Context, req *tfprotov5.ValidateResourceTypeConfigRequest) (*tfprotov5.ValidateResourceTypeConfigResponse, error) {
	r, d := s.lookup(req.TypeName)
	if d != nil {
		return &tfprotov5.ValidateResourceTypeConfigResponse{Diagnostics: []*tfprotov5.Diagnostic{d}}, nil
	}
	cfgVal, err := decode(req.Config, r.objType())
	if err != nil {
		return nil, err
	}
	cfg, err := asObject(cfgVal)
	if err != nil {
		return nil, err
	}
	return &tfprotov5.ValidateResourceTypeConfigResponse{Diagnostics: r.validateConfig(cfg)}, nil
}

func (s *Server) PlanResourceChange(_ context.Context, req *tfprotov5.PlanResourceChangeRequest) (*tfprotov5.PlanResourceChangeResponse, error) {
	r, d := s.lookup(req.TypeName)
	if d != nil {
		return &tfprotov5.PlanResourceChangeResponse{Diagnostics: []*tfprotov5.Diagnostic{d}}, nil
	}
	// A null ProposedNewState means a delete: plan a null state.
	proposed, err := decode(req.ProposedNewState, r.objType())
	if err != nil {
		return nil, err
	}
	if proposed.IsNull() {
		dv, err := encode(r.objType(), proposed)
		if err != nil {
			return nil, err
		}
		return &tfprotov5.PlanResourceChangeResponse{PlannedState: dv}, nil
	}
	cfgVal, err := decode(req.Config, r.objType())
	if err != nil {
		return nil, err
	}
	cfg, err := asObject(cfgVal)
	if err != nil {
		return nil, err
	}
	planned := r.planned(cfg)
	dv, err := encode(r.objType(), planned)
	if err != nil {
		return nil, err
	}
	return &tfprotov5.PlanResourceChangeResponse{PlannedState: dv}, nil
}

func (s *Server) ApplyResourceChange(_ context.Context, req *tfprotov5.ApplyResourceChangeRequest) (*tfprotov5.ApplyResourceChangeResponse, error) {
	r, d := s.lookup(req.TypeName)
	if d != nil {
		return &tfprotov5.ApplyResourceChangeResponse{Diagnostics: []*tfprotov5.Diagnostic{d}}, nil
	}
	// A null PlannedState means a destroy: return null new state.
	planned, err := decode(req.PlannedState, r.objType())
	if err != nil {
		return nil, err
	}
	if planned.IsNull() {
		dv, err := encode(r.objType(), tftypes.NewValue(r.objType(), nil))
		if err != nil {
			return nil, err
		}
		return &tfprotov5.ApplyResourceChangeResponse{NewState: dv}, nil
	}
	cfgVal, err := decode(req.Config, r.objType())
	if err != nil {
		return nil, err
	}
	cfg, err := asObject(cfgVal)
	if err != nil {
		return nil, err
	}
	if diags := r.validateConfig(cfg); len(diags) > 0 {
		return &tfprotov5.ApplyResourceChangeResponse{Diagnostics: diags}, nil
	}
	newState, diags := r.applied(cfg, s.counter.Next())
	if len(diags) > 0 {
		return &tfprotov5.ApplyResourceChangeResponse{Diagnostics: diags}, nil
	}
	dv, err := encode(r.objType(), newState)
	if err != nil {
		return nil, err
	}
	return &tfprotov5.ApplyResourceChangeResponse{NewState: dv}, nil
}

// ReadResource reconciles state. The fakes have no external store, so state is
// returned unchanged (refresh is a no-op that preserves the round trip).
func (s *Server) ReadResource(_ context.Context, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	return &tfprotov5.ReadResourceResponse{NewState: req.CurrentState}, nil
}

// UpgradeResourceState passes raw state through decoded at the current schema.
func (s *Server) UpgradeResourceState(_ context.Context, req *tfprotov5.UpgradeResourceStateRequest) (*tfprotov5.UpgradeResourceStateResponse, error) {
	r, d := s.lookup(req.TypeName)
	if d != nil {
		return &tfprotov5.UpgradeResourceStateResponse{Diagnostics: []*tfprotov5.Diagnostic{d}}, nil
	}
	if req.RawState == nil {
		return &tfprotov5.UpgradeResourceStateResponse{}, nil
	}
	v, err := req.RawState.Unmarshal(r.objType())
	if err != nil {
		return nil, err
	}
	dv, err := encode(r.objType(), v)
	if err != nil {
		return nil, err
	}
	return &tfprotov5.UpgradeResourceStateResponse{UpgradedState: dv}, nil
}

// --- unimplemented RPCs (a fake provider does not need these) ---------------

func (s *Server) GetResourceIdentitySchemas(_ context.Context, _ *tfprotov5.GetResourceIdentitySchemasRequest) (*tfprotov5.GetResourceIdentitySchemasResponse, error) {
	return &tfprotov5.GetResourceIdentitySchemasResponse{}, nil
}

func (s *Server) UpgradeResourceIdentity(_ context.Context, _ *tfprotov5.UpgradeResourceIdentityRequest) (*tfprotov5.UpgradeResourceIdentityResponse, error) {
	return &tfprotov5.UpgradeResourceIdentityResponse{Diagnostics: []*tfprotov5.Diagnostic{unimplemented("UpgradeResourceIdentity")}}, nil
}

func (s *Server) ImportResourceState(_ context.Context, _ *tfprotov5.ImportResourceStateRequest) (*tfprotov5.ImportResourceStateResponse, error) {
	return &tfprotov5.ImportResourceStateResponse{Diagnostics: []*tfprotov5.Diagnostic{unimplemented("ImportResourceState")}}, nil
}

func (s *Server) MoveResourceState(_ context.Context, _ *tfprotov5.MoveResourceStateRequest) (*tfprotov5.MoveResourceStateResponse, error) {
	return &tfprotov5.MoveResourceStateResponse{Diagnostics: []*tfprotov5.Diagnostic{unimplemented("MoveResourceState")}}, nil
}

func (s *Server) GenerateResourceConfig(_ context.Context, _ *tfprotov5.GenerateResourceConfigRequest) (*tfprotov5.GenerateResourceConfigResponse, error) {
	return &tfprotov5.GenerateResourceConfigResponse{Diagnostics: []*tfprotov5.Diagnostic{unimplemented("GenerateResourceConfig")}}, nil
}

func (s *Server) ValidateDataSourceConfig(_ context.Context, _ *tfprotov5.ValidateDataSourceConfigRequest) (*tfprotov5.ValidateDataSourceConfigResponse, error) {
	return &tfprotov5.ValidateDataSourceConfigResponse{Diagnostics: []*tfprotov5.Diagnostic{unimplemented("ValidateDataSourceConfig")}}, nil
}

func (s *Server) ReadDataSource(_ context.Context, _ *tfprotov5.ReadDataSourceRequest) (*tfprotov5.ReadDataSourceResponse, error) {
	return &tfprotov5.ReadDataSourceResponse{Diagnostics: []*tfprotov5.Diagnostic{unimplemented("ReadDataSource")}}, nil
}

func (s *Server) GetFunctions(_ context.Context, _ *tfprotov5.GetFunctionsRequest) (*tfprotov5.GetFunctionsResponse, error) {
	return &tfprotov5.GetFunctionsResponse{}, nil
}

func (s *Server) CallFunction(_ context.Context, _ *tfprotov5.CallFunctionRequest) (*tfprotov5.CallFunctionResponse, error) {
	return &tfprotov5.CallFunctionResponse{Error: &tfprotov5.FunctionError{Text: "this fake provider implements no functions"}}, nil
}

func (s *Server) ValidateEphemeralResourceConfig(_ context.Context, _ *tfprotov5.ValidateEphemeralResourceConfigRequest) (*tfprotov5.ValidateEphemeralResourceConfigResponse, error) {
	return &tfprotov5.ValidateEphemeralResourceConfigResponse{Diagnostics: []*tfprotov5.Diagnostic{unimplemented("ValidateEphemeralResourceConfig")}}, nil
}

func (s *Server) OpenEphemeralResource(_ context.Context, _ *tfprotov5.OpenEphemeralResourceRequest) (*tfprotov5.OpenEphemeralResourceResponse, error) {
	return &tfprotov5.OpenEphemeralResourceResponse{Diagnostics: []*tfprotov5.Diagnostic{unimplemented("OpenEphemeralResource")}}, nil
}

func (s *Server) RenewEphemeralResource(_ context.Context, _ *tfprotov5.RenewEphemeralResourceRequest) (*tfprotov5.RenewEphemeralResourceResponse, error) {
	return &tfprotov5.RenewEphemeralResourceResponse{Diagnostics: []*tfprotov5.Diagnostic{unimplemented("RenewEphemeralResource")}}, nil
}

func (s *Server) CloseEphemeralResource(_ context.Context, _ *tfprotov5.CloseEphemeralResourceRequest) (*tfprotov5.CloseEphemeralResourceResponse, error) {
	return &tfprotov5.CloseEphemeralResourceResponse{Diagnostics: []*tfprotov5.Diagnostic{unimplemented("CloseEphemeralResource")}}, nil
}
