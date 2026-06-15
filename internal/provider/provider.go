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

// ResourceSchema is a resource type's normalized schema.
type ResourceSchema struct {
	TypeName string
	Attrs    []Attr
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
}

type PlanResult struct {
	// PlannedState is an opaque backend handle carried into Apply.
	PlannedState interface{}
	// UnknownAfterApply lists attributes unknown in the planned state.
	UnknownAfterApply []string
	Diagnostics       []Diagnostic
}

// ApplyRequest / ApplyResult: apply one planned resource.
type ApplyRequest struct {
	Schema       ResourceSchema
	TypeName     string
	ResolvedCfg  map[string]interface{}
	PlannedState interface{} // from PlanResult
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
	// ListResourceTypes returns the provider's resource type names (sorted).
	ListResourceTypes(ctx context.Context) ([]string, error)
	// GetSchema returns the schema for one resource type.
	GetSchema(ctx context.Context, resourceType string) (ResourceSchema, error)
	Plan(ctx context.Context, req PlanRequest) (PlanResult, error)
	Apply(ctx context.Context, req ApplyRequest) (ApplyResult, error)
	Read(ctx context.Context, req ReadRequest) (ReadResult, error)
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
