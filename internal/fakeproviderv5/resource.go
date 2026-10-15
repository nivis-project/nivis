// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package fakeproviderv5

import (
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
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
// below turn this into tfprotov5 behavior.
// Rename is one attribute rename introduced AT a schema version: state written
// at any version below `AtVersion` carries the value under `From`, and the
// current schema calls it `To`.
type Rename struct {
	AtVersion int64
	From      string
	To        string
}

type Resource struct {
	TypeName string
	Attrs    map[string]Attr
	// Renames declares the schema changes this resource has been through, so
	// UpgradeResourceState can carry an old value across rather than dropping
	// it. A fake with none upgrades by doing nothing, which is the honest
	// behaviour for a resource whose schema never changed.
	Renames []Rename
	// Apply receives the known input values (by attr name; absent/null inputs
	// are omitted) and the counter value for this apply, and returns the
	// computed string values keyed by attr name. Must be pure and deterministic.
	Apply func(inputs map[string]string, counter int64) (computed map[string]string, diags []*tfprotov5.Diagnostic)
}

// objType returns the tftypes.Object type for the whole resource (all attrs).
func (r Resource) objType() tftypes.Object {
	attrs := make(map[string]tftypes.Type, len(r.Attrs))
	for name, a := range r.Attrs {
		attrs[name] = a.Type
	}
	return objectType(attrs)
}

// schema renders the tfprotov5 schema for this resource.
func (r Resource) schema() *tfprotov5.Schema {
	var sas []*tfprotov5.SchemaAttribute
	for name, a := range r.Attrs {
		sas = append(sas, &tfprotov5.SchemaAttribute{
			Name:     name,
			Type:     a.Type,
			Required: a.Required,
			Optional: a.Optional,
			Computed: a.Computed,
		})
	}
	v := schemaVersion()
	return &tfprotov5.Schema{
		Version: v,
		Block:   &tfprotov5.SchemaBlock{Version: v, Attributes: sas},
	}
}

// validateConfig checks required inputs are set. Returns diagnostics.
func (r Resource) validateConfig(cfg map[string]tftypes.Value) []*tfprotov5.Diagnostic {
	var diags []*tfprotov5.Diagnostic
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
func (r Resource) applied(cfg, planned map[string]tftypes.Value, counter int64) (tftypes.Value, []*tfprotov5.Diagnostic) {
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
		if d.Severity == tfprotov5.DiagnosticSeverityError {
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

// requiresReplace lists the ForceNew attrs whose PLANNED value differs from the
// prior value, which is what a real provider compares: terraform-plugin-framework
// replaces when !PlanValue.Equal(StateValue), and SDKv2 sets RequiresNew on
// `Old != New || NewComputed`. A null prior is a create, which never replaces.
//
// There is deliberately no exclusion for a null or unknown planned value. An
// unknown differs from a known prior and must replace, which is what a cascade
// through an unresolved reference depends on; a planned null against a known
// prior is a ForceNew attr removed from config, which must replace too. An unset
// optional-computed attr does not replace without any exclusion, because its
// planned value carries the prior forward and the two compare equal.
func (r Resource) requiresReplace(planned, prior map[string]tftypes.Value) []*tftypes.AttributePath {
	if prior == nil {
		return nil
	}
	var paths []*tftypes.AttributePath
	for name, a := range r.Attrs {
		if !a.ForceNew {
			continue
		}
		if !valueAt(planned, name, a.Type).Equal(valueAt(prior, name, a.Type)) {
			paths = append(paths, tftypes.NewAttributePath().WithAttributeName(name))
		}
	}
	return paths
}

// valueAt reads an attr from a decoded object, treating an absent key as null so
// a missing entry and an explicit null compare equal.
func valueAt(attrs map[string]tftypes.Value, name string, t tftypes.Type) tftypes.Value {
	if v, ok := attrs[name]; ok {
		return v
	}
	return tftypes.NewValue(t, nil)
}

// --- schema version (test-only seam) -----------------------------------------

// schemaVersion is the version a fake reports for its resources. It defaults to
// 1, which is what the fakes have always hardcoded, so nothing changes unless a
// test asks. TERRAE_NIVIS_FAKE_SCHEMA_VERSION selects another.
//
// It is selectable at RUN time because that is the only way a hermetic test can
// produce state written at an older version and then meet a provider declaring
// a newer one, within one run of the suite. The fakes are separate binaries, so
// the choice has to cross a process boundary, and the environment is the channel
// already used for TERRAE_NIVIS_FAKE_COUNTER and TERRAE_NIVIS_FAKE_FAIL.
func schemaVersion() int64 {
	if s := os.Getenv("TERRAE_NIVIS_FAKE_SCHEMA_VERSION"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			return v
		}
	}
	return 1
}

// renamesFrom returns the renames that apply to state written at `from`: every
// rename introduced at a version ABOVE it. State at the current version gets
// none, which is why an equal-version upgrade is a no-op even when one is asked
// for.
func (r Resource) renamesFrom(from int64) []Rename {
	var out []Rename
	for _, ren := range r.Renames {
		if ren.AtVersion > from {
			out = append(out, ren)
		}
	}
	return out
}

// encodeAttrs builds a DynamicValue from a plain attribute map, dropping keys
// the current schema no longer has and nulling those it gained. That is correct
// HERE, unlike in the executor: an upgrade's whole job is to reshape old state
// to the current schema, and anything the renames did not carry across is gone
// by the provider's own declaration.
func encodeAttrs(objType tftypes.Object, decl map[string]Attr, attrs map[string]interface{}) (*tfprotov5.DynamicValue, error) {
	vals := map[string]tftypes.Value{}
	for name, a := range decl {
		raw, ok := attrs[name]
		if !ok || raw == nil {
			vals[name] = tftypes.NewValue(a.Type, nil)
			continue
		}
		s, isStr := raw.(string)
		if !isStr {
			vals[name] = tftypes.NewValue(a.Type, nil)
			continue
		}
		vals[name] = tftypes.NewValue(a.Type, s)
	}
	return encode(objType, tftypes.NewValue(objType, vals))
}
