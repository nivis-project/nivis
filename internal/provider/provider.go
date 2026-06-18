// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package provider is the version-neutral seam between the executor and a
// provider plugin. plan/apply/destroy/refresh/codegen depend on Client and the
// normalized types here; protocol-specific backends (v5, v6) live in
// subpackages and never leak protobuf to callers.
package provider

import "context"

// Severity classifies a diagnostic.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
)

// Diagnostic is a normalized provider diagnostic.
type Diagnostic struct {
	Severity Severity
	Summary  string
	Detail   string
}

// Attr is one resource attribute in the normalized schema (type kind + role
// flags). TypeKind is a coarse descriptor (string/number/bool/list/set/map/
// object/dynamic) sufficient for codegen; the backend retains any finer detail
// it needs internally via ResourceSchema.Raw.
type Attr struct {
	Name      string
	TypeKind  string
	Required  bool
	Optional  bool
	Computed  bool
	Sensitive bool
}

// BlockNesting is how a nested block is nested in config/state.
type BlockNesting string

const (
	BlockSingle BlockNesting = "single" // a single attrset
	BlockList   BlockNesting = "list"   // a list of attrsets ([ { ... } ])
	BlockSet    BlockNesting = "set"    // a set of attrsets ([ { ... } ])
	BlockMap    BlockNesting = "map"    // a map of attrsets ({ k = { ... }; })
)

// NestedBlock is one nested block of a resource: its name, nesting mode, inner
// attributes, and any blocks nested within it (recursively). Codegen uses this
// to emit a constructor argument with the correct shape per nesting, so authors
// never guess list-vs-single.
type NestedBlock struct {
	Name    string
	Nesting BlockNesting
	Attrs   []Attr
	Blocks  []NestedBlock
}

// ResourceSchema is a resource type's normalized schema.
type ResourceSchema struct {
	TypeName string
	Attrs    []Attr
	Blocks   []NestedBlock // nested blocks (with nesting mode), for codegen
	// raw is an opaque, backend-specific handle (e.g. the parsed object type)
	// the backend needs to encode/decode values for this resource. Callers do
	// not interpret it; they pass it back via PlanRequest/ApplyRequest.Schema.
	Raw interface{}
}

// PlanRequest / PlanResult: plan one resource.
type PlanRequest struct {
	Schema      ResourceSchema
	TypeName    string
	ResolvedCfg map[string]interface{} // unresolved __ref/__derived leaves => unknown
	// Prior is the resource's stored attributes, or nil when the resource is new.
	// The backend sends it as PriorState so the provider judges create vs. update
	// vs. replace; nil means a null prior state (a create).
	Prior map[string]interface{}
}

type PlanResult struct {
	// PlannedState is an opaque backend handle carried into Apply.
	PlannedState interface{}
	// UnknownAfterApply lists attributes unknown in the planned state.
	UnknownAfterApply []string
	// RequiresReplace is true when the provider's plan requires destroying and
	// recreating the resource (a force-new attribute changed) rather than an
	// in-place update. Always false for a create (nil prior state).
	RequiresReplace bool
	// NoOp is true when there is prior state and the planned state equals it with
	// nothing unknown and no replace — i.e. nothing to do. The executor skips
	// ApplyResourceChange for a no-op. Always false for a create.
	NoOp        bool
	Diagnostics []Diagnostic
}

// ApplyRequest / ApplyResult: apply one planned resource.
type ApplyRequest struct {
	Schema       ResourceSchema
	TypeName     string
	ResolvedCfg  map[string]interface{}
	PlannedState interface{} // from PlanResult
	// Prior is the resource's stored attributes, or nil for a create. The backend
	// sends it as PriorState so an in-place update is applied against real prior
	// state (not a null state, which would be a create).
	Prior map[string]interface{}
}

type ApplyResult struct {
	// Attrs are the now-known output attributes.
	Attrs       map[string]interface{}
	Diagnostics []Diagnostic
}

// ReadRequest / ReadResult: reconcile one resource from stored state.
type ReadRequest struct {
	Schema   ResourceSchema
	TypeName string
	Stored   map[string]interface{}
}

type ReadResult struct {
	Attrs       map[string]interface{}
	Diagnostics []Diagnostic
}

// ReadDataSourceRequest / ReadDataSourceResult: read a datasource from its
// (fully-known) config. Unlike ReadRequest (which sends prior state), this sends
// the resolved config and returns the datasource's computed attributes.
type ReadDataSourceRequest struct {
	Schema      ResourceSchema
	TypeName    string
	ResolvedCfg map[string]interface{}
}

type ReadDataSourceResult struct {
	Attrs       map[string]interface{}
	Diagnostics []Diagnostic
}

// DestroyRequest: delete one resource given its stored state.
type DestroyRequest struct {
	Schema   ResourceSchema
	TypeName string
	Stored   map[string]interface{}
}

type DestroyResult struct {
	Diagnostics []Diagnostic
}

// Client is the version-neutral provider interface the executor uses.
type Client interface {
	// Configure passes provider-level configuration to the provider, which most
	// real providers require before any plan/apply. config is keyed by the
	// provider config schema's attribute names; attributes absent from the map
	// are sent as null so the provider applies its own defaults (e.g. the AWS
	// SDK credential/region chain from the environment). Must be called once,
	// before Plan/Apply/Read/Destroy. An all-empty config is a valid no-op for
	// providers that need no configuration (the fakes).
	Configure(ctx context.Context, config map[string]interface{}) error
	// ListResourceTypes returns the provider's resource type names (sorted).
	ListResourceTypes(ctx context.Context) ([]string, error)
	// GetSchema returns the schema for one resource type.
	GetSchema(ctx context.Context, resourceType string) (ResourceSchema, error)
	// GetDataSourceSchema returns the schema for one datasource type.
	GetDataSourceSchema(ctx context.Context, dataSourceType string) (ResourceSchema, error)
	Plan(ctx context.Context, req PlanRequest) (PlanResult, error)
	Apply(ctx context.Context, req ApplyRequest) (ApplyResult, error)
	Read(ctx context.Context, req ReadRequest) (ReadResult, error)
	// ReadDataSource reads a datasource from its resolved config.
	ReadDataSource(ctx context.Context, req ReadDataSourceRequest) (ReadDataSourceResult, error)
	Destroy(ctx context.Context, req DestroyRequest) (DestroyResult, error)
}

// DiagError converts error-severity diagnostics into a Go error (nil if none).
func DiagError(diags []Diagnostic) error {
	var msgs []string
	for _, d := range diags {
		if d.Severity == SeverityError {
			s := d.Summary
			if d.Detail != "" {
				s += ": " + d.Detail
			}
			msgs = append(msgs, s)
		}
	}
	if len(msgs) == 0 {
		return nil
	}
	return &DiagErr{Messages: msgs}
}

// DiagErr is an aggregated provider-diagnostic error.
type DiagErr struct{ Messages []string }

func (e *DiagErr) Error() string {
	out := "provider diagnostics: "
	for i, m := range e.Messages {
		if i > 0 {
			out += "; "
		}
		out += m
	}
	return out
}
