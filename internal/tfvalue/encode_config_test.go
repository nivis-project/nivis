// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package tfvalue

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// decodeConfig unmarshals what EncodeConfig produced, so these tests pin the
// value on the wire rather than the Go call that built it.
func decodeConfig(t *testing.T, objType tftypes.Object, config map[string]interface{}) map[string]tftypes.Value {
	t.Helper()
	dv, err := EncodeConfig(objType, config)
	if err != nil {
		t.Fatalf("EncodeConfig: %v", err)
	}
	v, err := tftypes.ValueFromMsgPack(dv.Msgpack, objType) //nolint:staticcheck // msgpack is the wire format
	if err != nil {
		t.Fatalf("ValueFromMsgPack: %v", err)
	}
	m := map[string]tftypes.Value{}
	if err := v.As(&m); err != nil {
		t.Fatalf("value.As: %v", err)
	}
	return m
}

// configObjType mixes computed and non-computed attributes of the same type, so
// a test can show the encoding does not depend on computedness.
var configObjType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
	"label":     tftypes.String, // optional input
	"tier":      tftypes.String, // optional + computed in the schema
	"id":        tftypes.String, // computed only
	"linked":    tftypes.String, // takes an unresolved reference
	"linked_oc": tftypes.String, // optional + computed, takes an unresolved ref
}}

func ref(id, attr string) map[string]interface{} {
	return map[string]interface{}{"__ref": map[string]interface{}{"id": id, "attr": attr}}
}

// TestEncodeConfigAbsentAttrsAreNull is the regression this change exists for.
// An attribute the configuration does not set is null, whether or not the
// schema marks it computed. Sending unknown instead suppresses a provider's
// schema default (bean nixform2-1mk0).
func TestEncodeConfigAbsentAttrsAreNull(t *testing.T) {
	got := decodeConfig(t, configObjType, map[string]interface{}{"label": "L"})

	for _, name := range []string{"tier", "id", "linked", "linked_oc"} {
		v := got[name]
		if !v.IsKnown() {
			t.Errorf("%s is unknown; an attribute absent from config must be null", name)
			continue
		}
		if !v.IsNull() {
			t.Errorf("%s = %v, want null", name, v)
		}
	}
}

func TestEncodeConfigComputednessDoesNotChangeTheEncoding(t *testing.T) {
	got := decodeConfig(t, configObjType, map[string]interface{}{})

	// label is not computed, tier and id are. All three are absent from config,
	// so all three must encode identically.
	for _, name := range []string{"label", "tier", "id"} {
		if v := got[name]; !v.IsKnown() || !v.IsNull() {
			t.Errorf("%s = %v, want null", name, v)
		}
	}
}

func TestEncodeConfigSetAttrsKeepTheirValue(t *testing.T) {
	got := decodeConfig(t, configObjType, map[string]interface{}{
		"label": "L",
		"tier":  "premium",
	})

	for name, want := range map[string]string{"label": "L", "tier": "premium"} {
		var s string
		if err := got[name].As(&s); err != nil {
			t.Fatalf("%s.As: %v", name, err)
		}
		if s != want {
			t.Errorf("%s = %q, want %q", name, s, want)
		}
	}
}

// TestEncodeConfigUnresolvedRefsAreUnknown is the other half of the contract:
// unknown in a Config object means the configuration set the attribute to
// something not resolved yet, and that is the only thing it may mean.
func TestEncodeConfigUnresolvedRefsAreUnknown(t *testing.T) {
	got := decodeConfig(t, configObjType, map[string]interface{}{
		"linked":    ref("other", "id"),
		"linked_oc": ref("other", "id"),
	})

	// linked_oc is optional+computed and linked is not; both carry a reference,
	// so both are unknown for the same reason.
	for _, name := range []string{"linked", "linked_oc"} {
		if got[name].IsKnown() {
			t.Errorf("%s = %v, want unknown: it holds an unresolved reference", name, got[name])
		}
	}
}

func TestEncodeConfigDerivedAndSensitiveRefsAreUnknown(t *testing.T) {
	for _, marker := range []string{"__derived", "__sensitiveRef"} {
		got := decodeConfig(t, configObjType, map[string]interface{}{
			"linked": map[string]interface{}{marker: map[string]interface{}{"x": "y"}},
		})
		if got["linked"].IsKnown() {
			t.Errorf("%s leaf encoded as %v, want unknown", marker, got["linked"])
		}
	}
}

// TestEncodeConfigTableDriven covers the four cases together, so the shape of
// the rule is visible in one place.
func TestEncodeConfigTableDriven(t *testing.T) {
	type want int
	const (
		wantNull want = iota
		wantUnknown
		wantKnown
	)

	cases := []struct {
		name   string
		config map[string]interface{}
		attr   string
		want   want
	}{
		{"absent non-computed", map[string]interface{}{}, "label", wantNull},
		{"absent optional-computed", map[string]interface{}{}, "tier", wantNull},
		{"absent computed-only", map[string]interface{}{}, "id", wantNull},
		{"set non-computed", map[string]interface{}{"label": "L"}, "label", wantKnown},
		{"set optional-computed", map[string]interface{}{"tier": "t"}, "tier", wantKnown},
		{"unresolved ref", map[string]interface{}{"linked": ref("o", "id")}, "linked", wantUnknown},
		{"unresolved ref on optional-computed", map[string]interface{}{"linked_oc": ref("o", "id")}, "linked_oc", wantUnknown},
		{"explicit null", map[string]interface{}{"tier": nil}, "tier", wantNull},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := decodeConfig(t, configObjType, c.config)[c.attr]
			switch c.want {
			case wantNull:
				if !v.IsKnown() || !v.IsNull() {
					t.Errorf("%s = %v, want null", c.attr, v)
				}
			case wantUnknown:
				if v.IsKnown() {
					t.Errorf("%s = %v, want unknown", c.attr, v)
				}
			case wantKnown:
				if !v.IsKnown() || v.IsNull() {
					t.Errorf("%s = %v, want a known value", c.attr, v)
				}
			}
		})
	}
}
