// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package gen

import (
	"context"
	"sort"

	"github.com/nivis-project/nivis/internal/provider"
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
	attrs := attrsFromProvider(sch.Attrs)
	sortAttrs(attrs)
	return Resource{Type: sch.TypeName, Attrs: attrs, Blocks: blocksFromProvider(sch.Blocks)}
}

func attrsFromProvider(in []provider.Attr) []Attr {
	var attrs []Attr
	for _, a := range in {
		attrs = append(attrs, Attr{
			Name:      a.Name,
			Type:      NixType{Kind: kindFromString(a.TypeKind)},
			Required:  a.Required,
			Optional:  a.Optional,
			Computed:  a.Computed,
			Sensitive: a.Sensitive,
		})
	}
	return attrs
}

// blocksFromProvider maps provider nested blocks to the gen model (recursively),
// sorted by name for deterministic emission.
func blocksFromProvider(in []provider.NestedBlock) []Block {
	var blocks []Block
	for _, b := range in {
		inner := attrsFromProvider(b.Attrs)
		sortAttrs(inner)
		blocks = append(blocks, Block{
			Name:    b.Name,
			Nesting: nestingFromProvider(b.Nesting),
			Attrs:   inner,
			Blocks:  blocksFromProvider(b.Blocks),
		})
	}
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].Name < blocks[j].Name })
	return blocks
}

func nestingFromProvider(n provider.BlockNesting) Nesting {
	switch n {
	case provider.BlockList:
		return NestList
	case provider.BlockSet:
		return NestSet
	case provider.BlockMap:
		return NestMap
	default:
		return NestSingle
	}
}

func kindFromString(s string) Kind {
	switch Kind(s) {
	case KindString, KindNumber, KindBool, KindList, KindSet, KindMap, KindObject:
		return Kind(s)
	default:
		return KindDynamic
	}
}
