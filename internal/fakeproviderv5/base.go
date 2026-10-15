// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package fakeproviderv5

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

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
// counter from TERRAE_NIVIS_FAKE_COUNTER.
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
	proposedAttrs, err := asObject(proposed)
	if err != nil {
		return nil, err
	}
	priorVal, err := decode(req.PriorState, r.objType())
	if err != nil {
		return nil, err
	}
	var priorAttrs map[string]tftypes.Value
	if !priorVal.IsNull() {
		priorAttrs, err = asObject(priorVal)
		if err != nil {
			return nil, err
		}
	}
	planned := r.planned(cfg, proposedAttrs)
	plannedAttrs, err := asObject(planned)
	if err != nil {
		return nil, err
	}
	dv, err := encode(r.objType(), planned)
	if err != nil {
		return nil, err
	}
	return &tfprotov5.PlanResourceChangeResponse{
		PlannedState:    dv,
		RequiresReplace: r.requiresReplace(plannedAttrs, priorAttrs),
	}, nil
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
	// Injection happens HERE, after the destroy branch: in the protocol a
	// destroy IS ApplyResourceChange with a null planned state, so checking
	// before this point would fail destroys too.
	if failInjected("apply", req.TypeName) {
		return &tfprotov5.ApplyResourceChangeResponse{Diagnostics: []*tfprotov5.Diagnostic{errDiag(
			"Injected apply failure",
			"TERRAE_NIVIS_FAKE_FAIL requested that apply fail for "+req.TypeName+".")}}, nil
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
	plannedAttrs, err := asObject(planned)
	if err != nil {
		return nil, err
	}
	newState, diags := r.applied(cfg, plannedAttrs, s.counter.Next())
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
// ReadResource echoes the state it is given, EXCEPT in the two cases an import
// turns on.
//
// An import stub (an id set, every other attribute null) is POPULATED, because
// that is what a real provider does: the import returns the identity and the
// read fills in the rest. Without this a fake cannot model the stub-then-read
// flow the executor has to handle.
//
// TERRAE_NIVIS_FAKE_IMPORT_MISSING names an id that reads as NOTHING, which is
// how a provider reports that no such object exists. The executor must refuse
// an import on that, rather than reading it as an out-of-band deletion and
// planning a create.
func (s *Server) ReadResource(_ context.Context, req *tfprotov5.ReadResourceRequest) (*tfprotov5.ReadResourceResponse, error) {
	r, d := s.lookup(req.TypeName)
	if d != nil {
		return &tfprotov5.ReadResourceResponse{Diagnostics: []*tfprotov5.Diagnostic{d}}, nil
	}
	cur, err := decode(req.CurrentState, r.objType())
	if err != nil {
		return nil, err
	}
	attrs, err := asObject(cur)
	if err != nil {
		return &tfprotov5.ReadResourceResponse{NewState: req.CurrentState}, nil
	}
	id := stringOf(attrs["id"])
	if missing := os.Getenv("TERRAE_NIVIS_FAKE_IMPORT_MISSING"); missing != "" && id == missing {
		dv, err := encode(r.objType(), tftypes.NewValue(r.objType(), nil))
		if err != nil {
			return nil, err
		}
		return &tfprotov5.ReadResourceResponse{NewState: dv}, nil
	}
	if !isImportStub(r, attrs) {
		return &tfprotov5.ReadResourceResponse{NewState: req.CurrentState}, nil
	}
	filled, diags := r.applied(attrs, attrs, 0)
	for _, dg := range diags {
		if dg.Severity == tfprotov5.DiagnosticSeverityError {
			return &tfprotov5.ReadResourceResponse{Diagnostics: diags}, nil
		}
	}
	dv, err := encode(r.objType(), filled)
	if err != nil {
		return nil, err
	}
	return &tfprotov5.ReadResourceResponse{NewState: dv}, nil
}

// isImportStub reports whether the state looks like what an import returns: an
// id, and nothing else set.
func isImportStub(r Resource, attrs map[string]tftypes.Value) bool {
	if !isSet(attrs, "id") {
		return false
	}
	for name := range r.Attrs {
		if name == "id" {
			continue
		}
		if isSet(attrs, name) {
			return false
		}
	}
	return true
}

func stringOf(v tftypes.Value) string {
	if !v.IsKnown() || v.IsNull() {
		return ""
	}
	var s string
	if err := v.As(&s); err != nil {
		return ""
	}
	return s
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
	// Decode the raw JSON OURSELVES rather than via RawState.Unmarshal against
	// the current object type. That helper interprets the bytes as a type the
	// caller names, and the whole point of an upgrade is that the state was
	// written under a DIFFERENT shape, which is exactly what the current type
	// cannot describe. Its own documentation warns the encoding is lossy.
	attrs := map[string]interface{}{}
	if len(req.RawState.JSON) > 0 {
		if err := json.Unmarshal(req.RawState.JSON, &attrs); err != nil {
			return nil, fmt.Errorf("upgrade %q: raw state is not JSON: %w", req.TypeName, err)
		}
	}
	// Apply the renames declared for the version the request names. The REQUEST
	// decides, not the fake: a fake that assumed a version would agree with a
	// caller that sent the wrong one.
	for _, ren := range r.renamesFrom(req.Version) {
		if v, ok := attrs[ren.From]; ok {
			attrs[ren.To] = v
			delete(attrs, ren.From)
		}
	}
	dv, err := encodeAttrs(r.objType(), r.Attrs, attrs)
	if err != nil {
		return nil, fmt.Errorf("upgrade %q: %w", req.TypeName, err)
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

// ImportResourceState models a passthrough importer: it returns a stub carrying
// only the identity, which the caller then READS to populate. That is what real
// providers do (schema.ImportStatePassthroughContext returns just the ID), and
// it is the behaviour the executor's import path has to handle.
//
// TERRAE_NIVIS_FAKE_IMPORT_COUNT makes it return several resources for one
// import, so the executor's refusal of that case can be exercised. A refusal
// nothing can trigger is a refusal nobody knows works.
//
// An id this fake does not recognise still imports: the stub carries it, and the
// READ that follows returns nothing, which is how a provider reports that the
// object does not exist. importKnown decides which ids are real.
func (s *Server) ImportResourceState(_ context.Context, req *tfprotov5.ImportResourceStateRequest) (*tfprotov5.ImportResourceStateResponse, error) {
	r, d := s.lookup(req.TypeName)
	if d != nil {
		return &tfprotov5.ImportResourceStateResponse{Diagnostics: []*tfprotov5.Diagnostic{d}}, nil
	}
	n := importCount()
	var out []*tfprotov5.ImportedResource
	for i := 0; i < n; i++ {
		vals := map[string]tftypes.Value{}
		for name, a := range r.Attrs {
			if name == "id" {
				vals[name] = tftypes.NewValue(a.Type, req.ID)
				continue
			}
			vals[name] = tftypes.NewValue(a.Type, nil)
		}
		dv, err := encode(r.objType(), tftypes.NewValue(r.objType(), vals))
		if err != nil {
			return nil, err
		}
		out = append(out, &tfprotov5.ImportedResource{TypeName: req.TypeName, State: dv})
	}
	return &tfprotov5.ImportResourceStateResponse{ImportedResources: out}, nil
}

// importCount is how many resources an import returns. One, unless
// TERRAE_NIVIS_FAKE_IMPORT_COUNT says otherwise.
func importCount() int {
	if s := os.Getenv("TERRAE_NIVIS_FAKE_IMPORT_COUNT"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			return v
		}
	}
	return 1
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

// --- failure injection (test-only) -------------------------------------------

// failInjection reads TERRAE_NIVIS_FAKE_FAIL, a comma-separated list of
// "<operation>:<typename>" entries (e.g. "apply:delta_thing,destroy:beta_record").
// Unset means no injection, which is why no existing test is affected.
//
// It exists because every fake succeeds at everything, so the executor's failure
// paths cannot be reached through a spawned provider. A failure path that cannot
// be exercised is one whose behaviour is assumed rather than known.
func failInjected(op, typeName string) bool {
	spec := os.Getenv("TERRAE_NIVIS_FAKE_FAIL")
	if spec == "" {
		return false
	}
	want := op + ":" + typeName
	for _, e := range strings.Split(spec, ",") {
		if strings.TrimSpace(e) == want {
			return true
		}
	}
	return false
}
