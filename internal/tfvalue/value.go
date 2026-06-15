// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package tfvalue bridges the executor's plain Go config maps and the wire
// representation the provider expects: a tfprotov6 DynamicValue (msgpack) whose
// values conform to the resource schema. Unresolved __ref/__derived leaves are
// encoded as the protocol unknown value (the IR contract's unknown rep).
package tfvalue

import (
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wearetechnative/terrae-nivis/internal/tfcodec"
	"github.com/wearetechnative/terrae-nivis/internal/tfplugin6"
)

// ObjectType builds the tftypes.Object for a schema block: one attribute per
// flat attribute (or its NestedType), plus one per nested block, typed by the
// block's nesting mode (SINGLE/GROUP -> object, LIST -> list, SET -> set,
// MAP -> map of the nested object). The full set is required for the value to
// conform — providers like AWS expect every config attribute, blocks included.
func ObjectType(block *tfplugin6.Schema_Block) (tftypes.Object, error) {
	attrs := map[string]tftypes.Type{}
	for _, a := range block.GetAttributes() {
		if nt := a.GetNestedType(); nt != nil {
			ot, err := nestedObjectType(nt)
			if err != nil {
				return tftypes.Object{}, fmt.Errorf("attr %q: %w", a.GetName(), err)
			}
			attrs[a.GetName()] = ot
			continue
		}
		t, err := tftypes.ParseJSONType(a.GetType())
		if err != nil {
			return tftypes.Object{}, fmt.Errorf("attr %q: parse type: %w", a.GetName(), err)
		}
		attrs[a.GetName()] = t
	}
	for _, b := range block.GetBlockTypes() {
		bt, err := blockType(b)
		if err != nil {
			return tftypes.Object{}, fmt.Errorf("block %q: %w", b.GetTypeName(), err)
		}
		attrs[b.GetTypeName()] = bt
	}
	return tftypes.Object{AttributeTypes: attrs}, nil
}

// nestedObjectType builds an object type from a Schema_Object (NestedType attr).
func nestedObjectType(o *tfplugin6.Schema_Object) (tftypes.Type, error) {
	attrs := map[string]tftypes.Type{}
	for _, a := range o.GetAttributes() {
		if nt := a.GetNestedType(); nt != nil {
			ot, err := nestedObjectType(nt)
			if err != nil {
				return nil, err
			}
			attrs[a.GetName()] = ot
			continue
		}
		t, err := tftypes.ParseJSONType(a.GetType())
		if err != nil {
			return nil, fmt.Errorf("attr %q: %w", a.GetName(), err)
		}
		attrs[a.GetName()] = t
	}
	return tftypes.Object{AttributeTypes: attrs}, nil
}

// blockType maps a nested block to its tftypes type per nesting mode.
func blockType(b *tfplugin6.Schema_NestedBlock) (tftypes.Type, error) {
	inner, err := ObjectType(b.GetBlock())
	if err != nil {
		return nil, err
	}
	switch b.GetNesting() {
	case tfplugin6.Schema_NestedBlock_LIST, tfplugin6.Schema_NestedBlock_GROUP:
		return tftypes.List{ElementType: inner}, nil
	case tfplugin6.Schema_NestedBlock_SET:
		return tftypes.Set{ElementType: inner}, nil
	case tfplugin6.Schema_NestedBlock_MAP:
		return tftypes.Map{ElementType: inner}, nil
	case tfplugin6.Schema_NestedBlock_SINGLE:
		return inner, nil
	default:
		// INVALID or unknown: treat as a single object (best effort).
		return inner, nil
	}
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
// type (scalars, collections, nested objects). Shared with the v5 backend via
// internal/tfcodec.
func goToValue(t tftypes.Type, raw interface{}) (tftypes.Value, error) {
	return tfcodec.GoToValue(t, raw)
}

// EncodeState builds a DynamicValue from a flat stored-attrs map (all values
// known), used as PriorState/CurrentState for destroy and refresh. Attributes in
// the schema but absent from attrs become null.
func EncodeState(objType tftypes.Object, attrs map[string]interface{}) (*tfplugin6.DynamicValue, error) {
	vals := map[string]tftypes.Value{}
	for name, attrType := range objType.AttributeTypes {
		raw, present := attrs[name]
		if !present {
			vals[name] = tftypes.NewValue(attrType, nil)
			continue
		}
		v, err := goToValue(attrType, raw)
		if err != nil {
			return nil, fmt.Errorf("attr %q: %w", name, err)
		}
		vals[name] = v
	}
	mp, err := tftypes.NewValue(objType, vals).MarshalMsgPack(objType) //nolint:staticcheck
	if err != nil {
		return nil, fmt.Errorf("marshal msgpack: %w", err)
	}
	return &tfplugin6.DynamicValue{Msgpack: mp}, nil
}

// NullState builds a null DynamicValue of the object type (the planned state for
// a destroy).
func NullState(objType tftypes.Object) (*tfplugin6.DynamicValue, error) {
	mp, err := tftypes.NewValue(objType, nil).MarshalMsgPack(objType) //nolint:staticcheck
	if err != nil {
		return nil, fmt.Errorf("marshal null msgpack: %w", err)
	}
	return &tfplugin6.DynamicValue{Msgpack: mp}, nil
}

// UnknownAttrs decodes a DynamicValue (a planned state) and lists the attribute
// names whose value is unknown (known only after apply).
func UnknownAttrs(objType tftypes.Object, dv *tfplugin6.DynamicValue) ([]string, error) {
	if dv == nil || len(dv.GetMsgpack()) == 0 {
		return nil, nil
	}
	v, err := tftypes.ValueFromMsgPack(dv.GetMsgpack(), objType)
	if err != nil {
		return nil, fmt.Errorf("decode planned state: %w", err)
	}
	m := map[string]tftypes.Value{}
	if err := v.As(&m); err != nil {
		return nil, err
	}
	var out []string
	for name, av := range m {
		if !av.IsKnown() {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
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
	return tfcodec.ValueToGo(v)
}
