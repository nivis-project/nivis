// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package gen

import (
	"sort"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// mapType converts a tftypes.Type into the codegen's NixType descriptor,
// recursing into collection element types and object attribute types. Unknown
// types fall back to KindDynamic (recorded, not guessed).
func mapType(t tftypes.Type) NixType {
	switch {
	case t == nil:
		return NixType{Kind: KindDynamic}
	case t.Is(tftypes.String):
		return NixType{Kind: KindString}
	case t.Is(tftypes.Number):
		return NixType{Kind: KindNumber}
	case t.Is(tftypes.Bool):
		return NixType{Kind: KindBool}
	}

	switch tt := t.(type) {
	case tftypes.List:
		e := mapType(tt.ElementType)
		return NixType{Kind: KindList, Elem: &e}
	case tftypes.Set:
		e := mapType(tt.ElementType)
		return NixType{Kind: KindSet, Elem: &e}
	case tftypes.Map:
		e := mapType(tt.ElementType)
		return NixType{Kind: KindMap, Elem: &e}
	case tftypes.Object:
		attrs := make(map[string]*NixType, len(tt.AttributeTypes))
		for name, at := range tt.AttributeTypes {
			n := mapType(at)
			attrs[name] = &n
		}
		return NixType{Kind: KindObject, Attrs: attrs}
	}
	return NixType{Kind: KindDynamic}
}

// nestedObjectType builds a NixType{object} from a schema NestedType's parsed
// attributes (block-style nested attributes that have no flat tftype).
func nestedObjectType(attrs []Attr) NixType {
	m := make(map[string]*NixType, len(attrs))
	for i := range attrs {
		t := attrs[i].Type
		m[attrs[i].Name] = &t
	}
	return NixType{Kind: KindObject, Attrs: m}
}

// sortAttrs orders attributes by name for deterministic emission.
func sortAttrs(attrs []Attr) {
	sort.Slice(attrs, func(i, j int) bool { return attrs[i].Name < attrs[j].Name })
}
