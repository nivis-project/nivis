// Copyright 2026 WeareTechnative B.V. and the nixform authors
// SPDX-License-Identifier: Apache-2.0

package gen

import (
	"context"
	"sort"

	"github.com/wearetechnative/nixform/internal/provider"
)

// Fetch returns the normalized resource models for codegen via the
// version-neutral provider.Client (works against any protocol the manager
// supports). It discovers the resource types from the provider and fetches each.
func Fetch(ctx context.Context, client provider.Client) ([]Resource, error) {
	types, err := client.ListResourceTypes(ctx)
	if err != nil {
		return nil, err
	}
	var out []Resource
	for _, rt := range types {
		sch, err := client.GetSchema(ctx, rt)
		if err != nil {
			return nil, err
		}
		out = append(out, fromSchema(sch))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Type < out[j].Type })
	return out, nil
}

// fromSchema converts a provider.ResourceSchema into the codegen model.
func fromSchema(sch provider.ResourceSchema) Resource {
	var attrs []Attr
	for _, a := range sch.Attrs {
		attrs = append(attrs, Attr{
			Name:      a.Name,
			Type:      NixType{Kind: kindFromString(a.TypeKind)},
			Required:  a.Required,
			Optional:  a.Optional,
			Computed:  a.Computed,
			Sensitive: a.Sensitive,
		})
	}
	sortAttrs(attrs)
	return Resource{Type: sch.TypeName, Attrs: attrs}
}

func kindFromString(s string) Kind {
	switch Kind(s) {
	case KindString, KindNumber, KindBool, KindList, KindSet, KindMap, KindObject:
		return Kind(s)
	default:
		return KindDynamic
	}
}
