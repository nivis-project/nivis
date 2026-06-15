// Copyright 2026 WeareTechnative B.V. and the nixform authors
// SPDX-License-Identifier: Apache-2.0

package v5

import (
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wearetechnative/nixform/internal/tfcodec"
	"github.com/wearetechnative/nixform/internal/tfplugin5"
)

// This is the v5 mirror of internal/tfvalue: the same encode/decode logic
// against the tfplugin5 DynamicValue/Schema types. The wire format (msgpack over
// tftypes) is identical between v5 and v6; only the protobuf message types differ.

func objectType(block *tfplugin5.Schema_Block) (tftypes.Object, error) {
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

func encodeConfig(objType tftypes.Object, computed map[string]bool, config map[string]interface{}) (*tfplugin5.DynamicValue, error) {
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
	mp, err := tftypes.NewValue(objType, vals).MarshalMsgPack(objType) //nolint:staticcheck
	if err != nil {
		return nil, fmt.Errorf("marshal msgpack: %w", err)
	}
	return &tfplugin5.DynamicValue{Msgpack: mp}, nil
}

func encodeState(objType tftypes.Object, attrs map[string]interface{}) (*tfplugin5.DynamicValue, error) {
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
	return &tfplugin5.DynamicValue{Msgpack: mp}, nil
}

func nullState(objType tftypes.Object) (*tfplugin5.DynamicValue, error) {
	mp, err := tftypes.NewValue(objType, nil).MarshalMsgPack(objType) //nolint:staticcheck
	if err != nil {
		return nil, fmt.Errorf("marshal null msgpack: %w", err)
	}
	return &tfplugin5.DynamicValue{Msgpack: mp}, nil
}

func unknownAttrs(objType tftypes.Object, dv *tfplugin5.DynamicValue) ([]string, error) {
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

func decodeState(objType tftypes.Object, dv *tfplugin5.DynamicValue) (map[string]interface{}, error) {
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

// goToValue / valueToGo delegate to the shared codec (internal/tfcodec). The
// wire format is identical between v5 and v6; only the DynamicValue/Schema
// protobuf wrappers differ.
func goToValue(t tftypes.Type, raw interface{}) (tftypes.Value, error) {
	return tfcodec.GoToValue(t, raw)
}

func valueToGo(v tftypes.Value) (interface{}, bool, error) {
	return tfcodec.ValueToGo(v)
}
