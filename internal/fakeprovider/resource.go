// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

package fakeprovider

import (
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Attr describes one resource attribute: its type and role. Exactly one of the
// role flags drives behavior: Computed attrs are unknown at plan and filled by
// Apply; Required/Optional attrs are user-supplied inputs.
type Attr struct {
	Type     tftypes.Type
	Required bool
	Optional bool
	Computed bool
}

// Resource is a fake resource definition: a type name, its attribute set, and a
// pure Apply func that computes the values of the Computed attributes from the
// (known) input attributes plus a counter value. The generic protocol methods
// below turn this into tfprotov6 behavior.
type Resource struct {
	TypeName string
	Attrs    map[string]Attr
	// Apply receives the known input values (by attr name; absent/null inputs
	// are omitted) and the counter value for this apply, and returns the
	// computed string values keyed by attr name. Must be pure and deterministic.
	Apply func(inputs map[string]string, counter int64) (computed map[string]string, diags []*tfprotov6.Diagnostic)
}

// objType returns the tftypes.Object type for the whole resource (all attrs).
func (r Resource) objType() tftypes.Object {
	attrs := make(map[string]tftypes.Type, len(r.Attrs))
	for name, a := range r.Attrs {
		attrs[name] = a.Type
	}
	return objectType(attrs)
}

// schema renders the tfprotov6 schema for this resource.
func (r Resource) schema() *tfprotov6.Schema {
	var sas []*tfprotov6.SchemaAttribute
	for name, a := range r.Attrs {
		sas = append(sas, &tfprotov6.SchemaAttribute{
			Name:     name,
			Type:     a.Type,
			Required: a.Required,
			Optional: a.Optional,
			Computed: a.Computed,
		})
	}
	return &tfprotov6.Schema{
		Version: 1,
		Block:   &tfprotov6.SchemaBlock{Version: 1, Attributes: sas},
	}
}

// validateConfig checks required inputs are set. Returns diagnostics.
func (r Resource) validateConfig(cfg map[string]tftypes.Value) []*tfprotov6.Diagnostic {
	var diags []*tfprotov6.Diagnostic
	for name, a := range r.Attrs {
		if !a.Required {
			continue
		}
		v, ok := cfg[name]
		if !ok || v.IsNull() {
			diags = append(diags, errDiag(
				"Missing required argument",
				"The argument \""+name+"\" is required, but no definition was found."))
		}
	}
	return diags
}

// planned builds the planned state: every Computed attribute is set to the
// tftypes unknown value (its value isn't known until apply); every input
// attribute is carried through from config (preserving null/known as given).
func (r Resource) planned(cfg map[string]tftypes.Value) tftypes.Value {
	out := make(map[string]tftypes.Value, len(r.Attrs))
	for name, a := range r.Attrs {
		if a.Computed && !a.Required && !a.Optional {
			// pure computed: unknown at plan
			out[name] = tftypes.NewValue(a.Type, tftypes.UnknownValue)
			continue
		}
		if a.Computed {
			// optional+computed: unknown unless the user supplied a value
			if v, ok := cfg[name]; ok && v.IsKnown() && !v.IsNull() {
				out[name] = v
			} else {
				out[name] = tftypes.NewValue(a.Type, tftypes.UnknownValue)
			}
			continue
		}
		if v, ok := cfg[name]; ok {
			out[name] = v
		} else {
			out[name] = tftypes.NewValue(a.Type, nil)
		}
	}
	return tftypes.NewValue(r.objType(), out)
}

// applied computes the final, fully-known state. Inputs are read from config;
// the Apply func fills computed attrs; all attrs end up known (no unknowns).
func (r Resource) applied(cfg map[string]tftypes.Value, counter int64) (tftypes.Value, []*tfprotov6.Diagnostic) {
	inputs := map[string]string{}
	for name, a := range r.Attrs {
		if a.Computed && !a.Optional {
			continue
		}
		if s, err := optString(cfg, name); err == nil && s != nil {
			inputs[name] = *s
		}
	}

	computed, diags := r.Apply(inputs, counter)
	for _, d := range diags {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			return tftypes.Value{}, diags
		}
	}

	out := make(map[string]tftypes.Value, len(r.Attrs))
	for name, a := range r.Attrs {
		if a.Computed {
			if v, ok := cfg[name]; ok && v.IsKnown() && !v.IsNull() {
				out[name] = v // user-supplied optional+computed value preserved
				continue
			}
			out[name] = tftypes.NewValue(a.Type, computed[name])
			continue
		}
		if v, ok := cfg[name]; ok {
			out[name] = v
		} else {
			out[name] = tftypes.NewValue(a.Type, nil)
		}
	}
	return tftypes.NewValue(r.objType(), out), diags
}
