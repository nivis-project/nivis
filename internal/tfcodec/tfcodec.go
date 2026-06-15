// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package tfcodec converts between plain Go values (the shape decoded from the
// IR JSON / produced for state) and tftypes.Value, recursively across scalars,
// collections, and nested objects. It is protocol-version-agnostic: the msgpack
// wire format over tftypes is identical for tfprotov5 and tfprotov6, so both
// backends share this codec. Only the DynamicValue/Schema protobuf wrappers
// differ, and those live in the per-version backends.
package tfcodec

import (
	"fmt"
	"math/big"
	"reflect"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// AttrsEqual reports whether two decoded attribute maps are value-equal. Both are
// produced by ValueToGo from the same object type, so numbers are *big.Float and
// nested values are []interface{} / map[string]interface{}; reflect.DeepEqual
// compares them faithfully (big.Float compared by its fields, which is exact for
// values that round-tripped through the same codec). Used to detect a no-op plan
// (planned state equals prior state).
func AttrsEqual(a, b map[string]interface{}) bool {
	return reflect.DeepEqual(a, b)
}

// KnownAttrsMatchPrior reports whether a planned state is a no-op against prior:
// every attribute whose planned value is KNOWN equals the prior value. Attributes
// listed in `unknown` (computed-after-apply, which the provider re-marks unknown
// on a re-plan of an unchanged resource — arn, etag, version_id, …) are skipped,
// since the provider keeps their prior values when nothing changed. An attribute
// that is known in the plan but differs from prior means a real change.
func KnownAttrsMatchPrior(planned, prior map[string]interface{}, unknown []string) bool {
	unk := make(map[string]bool, len(unknown))
	for _, u := range unknown {
		unk[u] = true
	}
	for k, pv := range planned {
		if unk[k] {
			continue // value is unknown-after-apply; not a change signal
		}
		if !reflect.DeepEqual(pv, prior[k]) {
			return false
		}
	}
	return true
}

// GoToValue converts a decoded-JSON Go value to a tftypes.Value of the given
// type: scalars (string/number/bool), collections (list/set/tuple from a
// []interface{}), and map/object (from a map[string]interface{}), recursing into
// element/attribute types. A nil raw becomes a null value of the type.
func GoToValue(t tftypes.Type, raw interface{}) (tftypes.Value, error) {
	if raw == nil {
		return tftypes.NewValue(t, nil), nil
	}
	switch {
	case t.Is(tftypes.String):
		s, ok := raw.(string)
		if !ok {
			return tftypes.Value{}, fmt.Errorf("expected string, got %T", raw)
		}
		return tftypes.NewValue(t, s), nil
	case t.Is(tftypes.Bool):
		b, ok := raw.(bool)
		if !ok {
			return tftypes.Value{}, fmt.Errorf("expected bool, got %T", raw)
		}
		return tftypes.NewValue(t, b), nil
	case t.Is(tftypes.Number):
		f, ok := raw.(float64)
		if !ok {
			return tftypes.Value{}, fmt.Errorf("expected number, got %T", raw)
		}
		return tftypes.NewValue(t, f), nil
	}

	switch tt := t.(type) {
	case tftypes.List:
		return sliceToValue(t, tt.ElementType, nil, raw)
	case tftypes.Set:
		return sliceToValue(t, tt.ElementType, nil, raw)
	case tftypes.Tuple:
		return tupleToValue(t, tt.ElementTypes, raw)
	case tftypes.Map:
		return mapToValue(t, func(string) tftypes.Type { return tt.ElementType }, raw)
	case tftypes.Object:
		return mapToValue(t, func(k string) tftypes.Type { return tt.AttributeTypes[k] }, raw)
	}
	return tftypes.Value{}, fmt.Errorf("unsupported attribute type %s", t)
}

// sliceToValue builds a list/set value from a []interface{}; for tuples,
// elemTypes gives a per-index type, else elemType is used for all elements.
func sliceToValue(t, elemType tftypes.Type, elemTypes []tftypes.Type, raw interface{}) (tftypes.Value, error) {
	items, ok := raw.([]interface{})
	if !ok {
		return tftypes.Value{}, fmt.Errorf("expected array for %s, got %T", t, raw)
	}
	out := make([]tftypes.Value, len(items))
	for i, item := range items {
		et := elemType
		if elemTypes != nil {
			et = elemTypes[i]
		}
		v, err := GoToValue(et, item)
		if err != nil {
			return tftypes.Value{}, fmt.Errorf("[%d]: %w", i, err)
		}
		out[i] = v
	}
	return tftypes.NewValue(t, out), nil
}

func tupleToValue(t tftypes.Type, elemTypes []tftypes.Type, raw interface{}) (tftypes.Value, error) {
	items, ok := raw.([]interface{})
	if !ok {
		return tftypes.Value{}, fmt.Errorf("expected array for %s, got %T", t, raw)
	}
	if len(items) != len(elemTypes) {
		return tftypes.Value{}, fmt.Errorf("tuple %s expects %d elements, got %d", t, len(elemTypes), len(items))
	}
	return sliceToValue(t, nil, elemTypes, raw)
}

// mapToValue builds a map/object value from a map[string]interface{}; typeOf
// gives the element/attribute type for a key.
func mapToValue(t tftypes.Type, typeOf func(string) tftypes.Type, raw interface{}) (tftypes.Value, error) {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return tftypes.Value{}, fmt.Errorf("expected object for %s, got %T", t, raw)
	}
	out := make(map[string]tftypes.Value, len(m))
	// For objects, every attribute in the type must be present; fill missing
	// ones with null so the value conforms to the type.
	if obj, isObj := t.(tftypes.Object); isObj {
		for name, at := range obj.AttributeTypes {
			if _, present := m[name]; !present {
				out[name] = tftypes.NewValue(at, nil)
			}
		}
	}
	for k, item := range m {
		et := typeOf(k)
		if et == nil {
			return tftypes.Value{}, fmt.Errorf("%s has no type for key %q", t, k)
		}
		v, err := GoToValue(et, item)
		if err != nil {
			return tftypes.Value{}, fmt.Errorf("[%q]: %w", k, err)
		}
		out[k] = v
	}
	return tftypes.NewValue(t, out), nil
}

// ValueToGo converts a tftypes.Value to a plain Go value. The second return is
// false when the value is unknown (callers skip those). Null becomes nil/true.
// Collections become []interface{}, map/object become map[string]interface{}.
func ValueToGo(v tftypes.Value) (interface{}, bool, error) {
	if !v.IsKnown() {
		return nil, false, nil
	}
	if v.IsNull() {
		return nil, true, nil
	}
	switch {
	case v.Type().Is(tftypes.String):
		var s string
		err := v.As(&s)
		return s, true, err
	case v.Type().Is(tftypes.Bool):
		var b bool
		err := v.As(&b)
		return b, true, err
	case v.Type().Is(tftypes.Number):
		// tftypes Number values decode into *big.Float, not *float64.
		bf := new(big.Float)
		if err := v.As(&bf); err != nil {
			return nil, false, err
		}
		f, _ := bf.Float64()
		return f, true, nil
	}

	switch v.Type().(type) {
	case tftypes.List, tftypes.Set, tftypes.Tuple:
		var elems []tftypes.Value
		if err := v.As(&elems); err != nil {
			return nil, false, err
		}
		out := make([]interface{}, 0, len(elems))
		for i, e := range elems {
			gv, known, err := ValueToGo(e)
			if err != nil {
				return nil, false, fmt.Errorf("[%d]: %w", i, err)
			}
			if known {
				out = append(out, gv)
			}
		}
		return out, true, nil
	case tftypes.Map, tftypes.Object:
		var m map[string]tftypes.Value
		if err := v.As(&m); err != nil {
			return nil, false, err
		}
		out := make(map[string]interface{}, len(m))
		for k, e := range m {
			gv, known, err := ValueToGo(e)
			if err != nil {
				return nil, false, fmt.Errorf("[%q]: %w", k, err)
			}
			if known {
				out[k] = gv
			}
		}
		return out, true, nil
	}
	return nil, false, fmt.Errorf("unsupported result type %s", v.Type())
}
