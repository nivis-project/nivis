// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package tfcodec

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// roundTrip encodes raw to a tftypes.Value of type t, then decodes it back, and
// returns the decoded Go value.
func roundTrip(t *testing.T, typ tftypes.Type, raw interface{}) interface{} {
	t.Helper()
	v, err := GoToValue(typ, raw)
	if err != nil {
		t.Fatalf("GoToValue(%s, %#v): %v", typ, raw, err)
	}
	got, known, err := ValueToGo(v)
	if err != nil {
		t.Fatalf("ValueToGo: %v", err)
	}
	if !known {
		t.Fatalf("value unexpectedly unknown")
	}
	return got
}

func TestScalarsRoundTrip(t *testing.T) {
	if got := roundTrip(t, tftypes.String, "hi"); got != "hi" {
		t.Errorf("string = %v", got)
	}
	if got := roundTrip(t, tftypes.Bool, true); got != true {
		t.Errorf("bool = %v", got)
	}
	if got := roundTrip(t, tftypes.Number, float64(42)); got != float64(42) {
		t.Errorf("number = %v", got)
	}
}

func TestListRoundTrip(t *testing.T) {
	typ := tftypes.List{ElementType: tftypes.String}
	got := roundTrip(t, typ, []interface{}{"a", "b"})
	if !reflect.DeepEqual(got, []interface{}{"a", "b"}) {
		t.Errorf("list = %#v", got)
	}
}

func TestSetRoundTrip(t *testing.T) {
	typ := tftypes.Set{ElementType: tftypes.Bool}
	got := roundTrip(t, typ, []interface{}{true, false})
	gs, ok := got.([]interface{})
	if !ok || len(gs) != 2 {
		t.Fatalf("set = %#v", got)
	}
}

func TestMapRoundTrip(t *testing.T) {
	typ := tftypes.Map{ElementType: tftypes.String}
	got := roundTrip(t, typ, map[string]interface{}{"env": "prod"})
	if !reflect.DeepEqual(got, map[string]interface{}{"env": "prod"}) {
		t.Errorf("map = %#v", got)
	}
}

func TestNestedObjectRoundTrip(t *testing.T) {
	typ := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":  tftypes.String,
		"ports": tftypes.List{ElementType: tftypes.Number},
	}}
	in := map[string]interface{}{
		"name":  "x",
		"ports": []interface{}{float64(80), float64(443)},
	}
	got := roundTrip(t, typ, in)
	m, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("object = %#v", got)
	}
	if m["name"] != "x" {
		t.Errorf("object.name = %v", m["name"])
	}
	if !reflect.DeepEqual(m["ports"], []interface{}{float64(80), float64(443)}) {
		t.Errorf("object.ports = %#v", m["ports"])
	}
}

func TestTupleRoundTrip(t *testing.T) {
	typ := tftypes.Tuple{ElementTypes: []tftypes.Type{tftypes.String, tftypes.Number}}
	got := roundTrip(t, typ, []interface{}{"a", float64(1)})
	if !reflect.DeepEqual(got, []interface{}{"a", float64(1)}) {
		t.Errorf("tuple = %#v", got)
	}
}

func TestObjectFillsMissingAttrsWithNull(t *testing.T) {
	// An object value missing an attribute must still conform to the type.
	typ := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"a": tftypes.String,
		"b": tftypes.String,
	}}
	v, err := GoToValue(typ, map[string]interface{}{"a": "x"}) // b omitted
	if err != nil {
		t.Fatalf("GoToValue with missing attr: %v", err)
	}
	got, _, err := ValueToGo(v)
	if err != nil {
		t.Fatal(err)
	}
	m := got.(map[string]interface{})
	if m["a"] != "x" {
		t.Errorf("a = %v", m["a"])
	}
	// b was null -> decoded as nil and kept (null is "known")
	if bv, present := m["b"]; !present || bv != nil {
		t.Errorf("b should be present and nil, got present=%v val=%v", present, bv)
	}
}

func TestUnsupportedTypeErrors(t *testing.T) {
	if _, err := GoToValue(tftypes.DynamicPseudoType, "x"); err == nil {
		t.Error("GoToValue must error on an unsupported type")
	}
}

func TestNullAndTypeMismatch(t *testing.T) {
	// nil -> null value, no error
	if _, err := GoToValue(tftypes.String, nil); err != nil {
		t.Errorf("nil should encode as null: %v", err)
	}
	// wrong Go type for a string attr -> error
	if _, err := GoToValue(tftypes.String, 123); err == nil {
		t.Error("a non-string for a string type must error")
	}
	// wrong shape for a list -> error
	if _, err := GoToValue(tftypes.List{ElementType: tftypes.String}, "not-a-list"); err == nil {
		t.Error("a non-array for a list type must error")
	}
}
