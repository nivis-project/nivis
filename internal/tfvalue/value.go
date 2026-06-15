// Package tfvalue bridges the executor's plain Go config maps and the wire
// representation the provider expects: a tfprotov6 DynamicValue (msgpack) whose
// values conform to the resource schema. Unresolved __ref/__derived leaves are
// encoded as the protocol unknown value (the IR contract's unknown rep).
package tfvalue

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wearetechnative/nixform/internal/tfplugin6"
)

// ObjectType builds the tftypes.Object for a resource from its schema block: one
// attribute per schema attribute, with the attribute's tftypes.Type parsed from
// the schema's JSON-encoded type.
func ObjectType(block *tfplugin6.Schema_Block) (tftypes.Object, error) {
	attrs := map[string]tftypes.Type{}
	for _, a := range block.GetAttributes() {
		t, err := tftypes.ParseJSONType(a.GetType())
		if err != nil {
			return tftypes.Object{}, fmt.Errorf("attr %q: parse type: %w", a.GetName(), err)
		}
		attrs[a.GetName()] = t
	}
	return tftypes.Object{AttributeTypes: attrs}, nil
}

// isUnresolved reports whether a config leaf is an unresolved marker (a __ref or
// __derived that did not get substituted). Such leaves become unknown values.
func isUnresolved(v interface{}) bool {
	m, ok := v.(map[string]interface{})
	if !ok {
		return false
	}
	_, ref := m["__ref"]
	_, der := m["__derived"]
	_, sref := m["__sensitiveRef"]
	return ref || der || sref
}

// EncodeConfig builds a DynamicValue (msgpack) for the given config against the
// resource object type. Attributes present in the schema but absent from config
// become null; computed attributes the user didn't set become unknown (the
// provider fills them); unresolved __ref/__derived leaves become unknown.
func EncodeConfig(objType tftypes.Object, computed map[string]bool, config map[string]interface{}) (*tfplugin6.DynamicValue, error) {
	vals := map[string]tftypes.Value{}
	for name, attrType := range objType.AttributeTypes {
		raw, present := config[name]
		switch {
		case !present && computed[name]:
			vals[name] = tftypes.NewValue(attrType, tftypes.UnknownValue)
		case !present:
			vals[name] = tftypes.NewValue(attrType, nil)
		case isUnresolved(raw):
			vals[name] = tftypes.NewValue(attrType, tftypes.UnknownValue)
		default:
			v, err := goToValue(attrType, raw)
			if err != nil {
				return nil, fmt.Errorf("attr %q: %w", name, err)
			}
			vals[name] = v
		}
	}
	objVal := tftypes.NewValue(objType, vals)
	mp, err := objVal.MarshalMsgPack(objType) //nolint:staticcheck // msgpack is the wire format
	if err != nil {
		return nil, fmt.Errorf("marshal msgpack: %w", err)
	}
	return &tfplugin6.DynamicValue{Msgpack: mp}, nil
}

// goToValue converts a decoded-JSON Go value to a tftypes.Value of the given
// type. Handles the scalar/collection shapes the fake providers use.
func goToValue(t tftypes.Type, raw interface{}) (tftypes.Value, error) {
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
	return tftypes.Value{}, fmt.Errorf("unsupported attribute type %s (PoC supports string/bool/number)", t)
}

// DecodeState reads a DynamicValue (the provider's NewState) into a flat Go attr
// map of known scalar values, against the resource object type.
func DecodeState(objType tftypes.Object, dv *tfplugin6.DynamicValue) (map[string]interface{}, error) {
	if dv == nil || len(dv.GetMsgpack()) == 0 {
		return map[string]interface{}{}, nil
	}
	v, err := tftypes.ValueFromMsgPack(dv.GetMsgpack(), objType)
	if err != nil {
		return nil, fmt.Errorf("decode msgpack: %w", err)
	}
	m := map[string]tftypes.Value{}
	if err := v.As(&m); err != nil {
		return nil, fmt.Errorf("decode object: %w", err)
	}
	out := make(map[string]interface{}, len(m))
	for name, av := range m {
		gv, known, err := valueToGo(av)
		if err != nil {
			return nil, fmt.Errorf("attr %q: %w", name, err)
		}
		if known {
			out[name] = gv
		}
	}
	return out, nil
}

// valueToGo converts a known scalar tftypes.Value to a Go value. The second
// return is false when the value is unknown (callers skip those).
func valueToGo(v tftypes.Value) (interface{}, bool, error) {
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
		var f float64
		err := v.As(&f)
		return f, true, err
	}
	return nil, false, fmt.Errorf("unsupported result type %s", v.Type())
}
