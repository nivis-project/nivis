// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package fakeprovider

import (
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Attr describes one resource attribute: its type and role. Exactly one of the
// role flags drives behavior: Computed attrs are unknown at plan and filled by
// Apply; Required/Optional attrs are user-supplied inputs. Default is a plain Go
// value applied at plan when an Optional+Computed attr is absent from config;
// nil means none. ForceNew marks an attr whose change against a non-null prior
// requires the resource to be replaced rather than updated in place.
type Attr struct {
	Type     tftypes.Type
	Required bool
	Optional bool
	Computed bool
	Default  interface{}
	ForceNew bool
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

// planned builds the planned state out of ProposedNewState (prior merged with
// config), as a real provider does; cfg only answers whether the user set an
// attribute. Pure computed attrs are unknown. An optional+computed attr takes
// the configured value, else a prior value carried in by proposed, else its
// declared Default, else unknown.
func (r Resource) planned(cfg, proposed map[string]tftypes.Value) tftypes.Value {
	out := make(map[string]tftypes.Value, len(r.Attrs))
	for name, a := range r.Attrs {
		if a.Computed && !a.Required && !a.Optional {
			out[name] = tftypes.NewValue(a.Type, tftypes.UnknownValue)
			continue
		}
		if a.Computed {
			switch {
			case isUnknown(cfg, name):
				out[name] = tftypes.NewValue(a.Type, tftypes.UnknownValue)
			case isSet(cfg, name):
				out[name] = cfg[name]
			case isSet(proposed, name):
				out[name] = proposed[name]
			case a.Default != nil:
				out[name] = tftypes.NewValue(a.Type, a.Default)
			default:
				out[name] = tftypes.NewValue(a.Type, tftypes.UnknownValue)
			}
			continue
		}
		if v, ok := proposed[name]; ok {
			out[name] = v
		} else {
			out[name] = tftypes.NewValue(a.Type, nil)
		}
	}
	return tftypes.NewValue(r.objType(), out)
}

// isSet reports whether attrs carries a known, non-null value for name.
func isSet(attrs map[string]tftypes.Value, name string) bool {
	v, ok := attrs[name]
	return ok && v.IsKnown() && !v.IsNull()
}

// isUnknown reports whether attrs carries an unknown value for name. In a config
// object that means the user set the attribute to an expression that is not
// resolved yet, which is not the same as leaving it unset: a default must not
// fire and the planned value stays unknown.
func isUnknown(attrs map[string]tftypes.Value, name string) bool {
	v, ok := attrs[name]
	return ok && !v.IsKnown()
}

// applied computes the final, fully-known state. Inputs are read from config;
// a computed attr already decided at plan (configured, carried from prior, or
// defaulted) keeps its planned value, and the Apply func fills the rest. All
// attrs end up known (no unknowns).
func (r Resource) applied(cfg, planned map[string]tftypes.Value, counter int64) (tftypes.Value, []*tfprotov6.Diagnostic) {
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
			if isSet(planned, name) {
				out[name] = planned[name]
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

// requiresReplace lists the ForceNew attrs whose configured value differs from
// the prior value. A null prior is a create, which never replaces; an attr
// absent from config is not a change, or every unset optional ForceNew attr
// would force a replacement on every plan.
func (r Resource) requiresReplace(cfg, prior map[string]tftypes.Value) []*tftypes.AttributePath {
	if prior == nil {
		return nil
	}
	var paths []*tftypes.AttributePath
	for name, a := range r.Attrs {
		if !a.ForceNew {
			continue
		}
		cv, ok := cfg[name]
		if !ok || cv.IsNull() || !cv.IsKnown() {
			continue
		}
		if pv, ok := prior[name]; !ok || !cv.Equal(pv) {
			paths = append(paths, tftypes.NewAttributePath().WithAttributeName(name))
		}
	}
	return paths
}
