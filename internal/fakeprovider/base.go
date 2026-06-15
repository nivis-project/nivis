package fakeprovider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Server is a tfprotov6.ProviderServer for a fake provider. It serves a fixed
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

var _ tfprotov6.ProviderServer = (*Server)(nil)

func unimplemented(rpc string) *tfprotov6.Diagnostic {
	return &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityError,
		Summary:  "Unimplemented",
		Detail:   fmt.Sprintf("This fake provider does not implement %s.", rpc),
	}
}

func (s *Server) lookup(typeName string) (Resource, *tfprotov6.Diagnostic) {
	r, ok := s.resources[typeName]
	if !ok {
		return Resource{}, errDiag("Unknown resource type",
			fmt.Sprintf("This provider has no resource type %q.", typeName))
	}
	return r, nil
}

// --- meaningful RPCs --------------------------------------------------------

func (s *Server) GetMetadata(_ context.Context, _ *tfprotov6.GetMetadataRequest) (*tfprotov6.GetMetadataResponse, error) {
	resp := &tfprotov6.GetMetadataResponse{ServerCapabilities: &tfprotov6.ServerCapabilities{}}
	for name := range s.resources {
		resp.Resources = append(resp.Resources, tfprotov6.ResourceMetadata{TypeName: name})
	}
	return resp, nil
}

func (s *Server) GetProviderSchema(_ context.Context, _ *tfprotov6.GetProviderSchemaRequest) (*tfprotov6.GetProviderSchemaResponse, error) {
	schemas := map[string]*tfprotov6.Schema{}
	for name, r := range s.resources {
		schemas[name] = r.schema()
	}
	return &tfprotov6.GetProviderSchemaResponse{
		Provider:        &tfprotov6.Schema{Version: 1, Block: &tfprotov6.SchemaBlock{Version: 1}},
		ResourceSchemas: schemas,
	}, nil
}

func (s *Server) ValidateProviderConfig(_ context.Context, _ *tfprotov6.ValidateProviderConfigRequest) (*tfprotov6.ValidateProviderConfigResponse, error) {
	return &tfprotov6.ValidateProviderConfigResponse{}, nil
}

func (s *Server) ConfigureProvider(_ context.Context, _ *tfprotov6.ConfigureProviderRequest) (*tfprotov6.ConfigureProviderResponse, error) {
	return &tfprotov6.ConfigureProviderResponse{}, nil
}

func (s *Server) StopProvider(_ context.Context, _ *tfprotov6.StopProviderRequest) (*tfprotov6.StopProviderResponse, error) {
	return &tfprotov6.StopProviderResponse{}, nil
}

func (s *Server) ValidateResourceConfig(_ context.Context, req *tfprotov6.ValidateResourceConfigRequest) (*tfprotov6.ValidateResourceConfigResponse, error) {
	r, d := s.lookup(req.TypeName)
	if d != nil {
		return &tfprotov6.ValidateResourceConfigResponse{Diagnostics: []*tfprotov6.Diagnostic{d}}, nil
	}
	cfgVal, err := decode(req.Config, r.objType())
	if err != nil {
		return nil, err
	}
	cfg, err := asObject(cfgVal)
	if err != nil {
		return nil, err
	}
	return &tfprotov6.ValidateResourceConfigResponse{Diagnostics: r.validateConfig(cfg)}, nil
}

func (s *Server) PlanResourceChange(_ context.Context, req *tfprotov6.PlanResourceChangeRequest) (*tfprotov6.PlanResourceChangeResponse, error) {
	r, d := s.lookup(req.TypeName)
	if d != nil {
		return &tfprotov6.PlanResourceChangeResponse{Diagnostics: []*tfprotov6.Diagnostic{d}}, nil
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
		return &tfprotov6.PlanResourceChangeResponse{PlannedState: dv}, nil
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
	return &tfprotov6.PlanResourceChangeResponse{PlannedState: dv}, nil
}

func (s *Server) ApplyResourceChange(_ context.Context, req *tfprotov6.ApplyResourceChangeRequest) (*tfprotov6.ApplyResourceChangeResponse, error) {
	r, d := s.lookup(req.TypeName)
	if d != nil {
		return &tfprotov6.ApplyResourceChangeResponse{Diagnostics: []*tfprotov6.Diagnostic{d}}, nil
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
		return &tfprotov6.ApplyResourceChangeResponse{NewState: dv}, nil
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
		return &tfprotov6.ApplyResourceChangeResponse{Diagnostics: diags}, nil
	}
	newState, diags := r.applied(cfg, s.counter.Next())
	if len(diags) > 0 {
		return &tfprotov6.ApplyResourceChangeResponse{Diagnostics: diags}, nil
	}
	dv, err := encode(r.objType(), newState)
	if err != nil {
		return nil, err
	}
	return &tfprotov6.ApplyResourceChangeResponse{NewState: dv}, nil
}

// ReadResource reconciles state. The fakes have no external store, so state is
// returned unchanged (refresh is a no-op that preserves the round trip).
func (s *Server) ReadResource(_ context.Context, req *tfprotov6.ReadResourceRequest) (*tfprotov6.ReadResourceResponse, error) {
	return &tfprotov6.ReadResourceResponse{NewState: req.CurrentState}, nil
}

// UpgradeResourceState passes raw state through decoded at the current schema.
func (s *Server) UpgradeResourceState(_ context.Context, req *tfprotov6.UpgradeResourceStateRequest) (*tfprotov6.UpgradeResourceStateResponse, error) {
	r, d := s.lookup(req.TypeName)
	if d != nil {
		return &tfprotov6.UpgradeResourceStateResponse{Diagnostics: []*tfprotov6.Diagnostic{d}}, nil
	}
	if req.RawState == nil {
		return &tfprotov6.UpgradeResourceStateResponse{}, nil
	}
	v, err := req.RawState.Unmarshal(r.objType())
	if err != nil {
		return nil, err
	}
	dv, err := encode(r.objType(), v)
	if err != nil {
		return nil, err
	}
	return &tfprotov6.UpgradeResourceStateResponse{UpgradedState: dv}, nil
}

// --- unimplemented RPCs (a fake provider does not need these) ---------------

func (s *Server) GetResourceIdentitySchemas(_ context.Context, _ *tfprotov6.GetResourceIdentitySchemasRequest) (*tfprotov6.GetResourceIdentitySchemasResponse, error) {
	return &tfprotov6.GetResourceIdentitySchemasResponse{}, nil
}

func (s *Server) UpgradeResourceIdentity(_ context.Context, _ *tfprotov6.UpgradeResourceIdentityRequest) (*tfprotov6.UpgradeResourceIdentityResponse, error) {
	return &tfprotov6.UpgradeResourceIdentityResponse{Diagnostics: []*tfprotov6.Diagnostic{unimplemented("UpgradeResourceIdentity")}}, nil
}

func (s *Server) ImportResourceState(_ context.Context, _ *tfprotov6.ImportResourceStateRequest) (*tfprotov6.ImportResourceStateResponse, error) {
	return &tfprotov6.ImportResourceStateResponse{Diagnostics: []*tfprotov6.Diagnostic{unimplemented("ImportResourceState")}}, nil
}

func (s *Server) MoveResourceState(_ context.Context, _ *tfprotov6.MoveResourceStateRequest) (*tfprotov6.MoveResourceStateResponse, error) {
	return &tfprotov6.MoveResourceStateResponse{Diagnostics: []*tfprotov6.Diagnostic{unimplemented("MoveResourceState")}}, nil
}

func (s *Server) GenerateResourceConfig(_ context.Context, _ *tfprotov6.GenerateResourceConfigRequest) (*tfprotov6.GenerateResourceConfigResponse, error) {
	return &tfprotov6.GenerateResourceConfigResponse{Diagnostics: []*tfprotov6.Diagnostic{unimplemented("GenerateResourceConfig")}}, nil
}

func (s *Server) ValidateDataResourceConfig(_ context.Context, _ *tfprotov6.ValidateDataResourceConfigRequest) (*tfprotov6.ValidateDataResourceConfigResponse, error) {
	return &tfprotov6.ValidateDataResourceConfigResponse{Diagnostics: []*tfprotov6.Diagnostic{unimplemented("ValidateDataResourceConfig")}}, nil
}

func (s *Server) ReadDataSource(_ context.Context, _ *tfprotov6.ReadDataSourceRequest) (*tfprotov6.ReadDataSourceResponse, error) {
	return &tfprotov6.ReadDataSourceResponse{Diagnostics: []*tfprotov6.Diagnostic{unimplemented("ReadDataSource")}}, nil
}

func (s *Server) GetFunctions(_ context.Context, _ *tfprotov6.GetFunctionsRequest) (*tfprotov6.GetFunctionsResponse, error) {
	return &tfprotov6.GetFunctionsResponse{}, nil
}

func (s *Server) CallFunction(_ context.Context, _ *tfprotov6.CallFunctionRequest) (*tfprotov6.CallFunctionResponse, error) {
	return &tfprotov6.CallFunctionResponse{Error: &tfprotov6.FunctionError{Text: "this fake provider implements no functions"}}, nil
}

func (s *Server) ValidateEphemeralResourceConfig(_ context.Context, _ *tfprotov6.ValidateEphemeralResourceConfigRequest) (*tfprotov6.ValidateEphemeralResourceConfigResponse, error) {
	return &tfprotov6.ValidateEphemeralResourceConfigResponse{Diagnostics: []*tfprotov6.Diagnostic{unimplemented("ValidateEphemeralResourceConfig")}}, nil
}

func (s *Server) OpenEphemeralResource(_ context.Context, _ *tfprotov6.OpenEphemeralResourceRequest) (*tfprotov6.OpenEphemeralResourceResponse, error) {
	return &tfprotov6.OpenEphemeralResourceResponse{Diagnostics: []*tfprotov6.Diagnostic{unimplemented("OpenEphemeralResource")}}, nil
}

func (s *Server) RenewEphemeralResource(_ context.Context, _ *tfprotov6.RenewEphemeralResourceRequest) (*tfprotov6.RenewEphemeralResourceResponse, error) {
	return &tfprotov6.RenewEphemeralResourceResponse{Diagnostics: []*tfprotov6.Diagnostic{unimplemented("RenewEphemeralResource")}}, nil
}

func (s *Server) CloseEphemeralResource(_ context.Context, _ *tfprotov6.CloseEphemeralResourceRequest) (*tfprotov6.CloseEphemeralResourceResponse, error) {
	return &tfprotov6.CloseEphemeralResourceResponse{Diagnostics: []*tfprotov6.Diagnostic{unimplemented("CloseEphemeralResource")}}, nil
}
