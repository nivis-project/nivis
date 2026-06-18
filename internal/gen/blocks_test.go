// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package gen

import (
	"strings"
	"testing"

	"github.com/wearetechnative/nivis/internal/provider"
)

// A resource with a list-nested and a single-nested block emits each as a
// constructor argument with the shape matching its nesting, plus a doc comment
// naming the nesting (so the generated file doubles as the argument reference and
// the list-vs-single mistake cannot be guessed).
func TestEmitNestedBlocks(t *testing.T) {
	r := Resource{
		Type:  "aws_instance",
		Attrs: []Attr{{Name: "ami", Required: true}, {Name: "id", Computed: true}},
		Blocks: []Block{
			{Name: "ingress", Nesting: NestList, Attrs: []Attr{{Name: "from_port", Required: true}}},
			{Name: "metadata_options", Nesting: NestSingle, Attrs: []Attr{{Name: "http_tokens", Optional: true}}},
			{Name: "tags_block", Nesting: NestMap, Attrs: []Attr{{Name: "v", Optional: true}}},
		},
	}
	out := Emit("aws", r)

	// list-nested -> defaults to [], documented as list-nested with [ { ... } ].
	if !strings.Contains(out, "ingress ? [ ]") {
		t.Errorf("list-nested block should default to []; got:\n%s", out)
	}
	if !strings.Contains(out, `Block "ingress" (list-nested)`) || !strings.Contains(out, "[ { ... } ]") {
		t.Errorf("missing list-nested doc comment; got:\n%s", out)
	}
	// single-nested -> defaults to null (one attrset).
	if !strings.Contains(out, "metadata_options ? null") {
		t.Errorf("single-nested block should default to null; got:\n%s", out)
	}
	if !strings.Contains(out, `Block "metadata_options" (single-nested)`) {
		t.Errorf("missing single-nested doc comment; got:\n%s", out)
	}
	// map-nested -> defaults to {}.
	if !strings.Contains(out, "tags_block ? { }") {
		t.Errorf("map-nested block should default to {}; got:\n%s", out)
	}
	// inner attr names are documented (the reference value).
	if !strings.Contains(out, "from_port") {
		t.Errorf("block inner attrs should be documented; got:\n%s", out)
	}
	// the block is wired into config (a real // merge, not dropped).
	if !strings.Contains(out, "ingress = ingress;") {
		t.Errorf("block should be merged into config; got:\n%s", out)
	}
}

// fromSchema maps provider nested blocks (with nesting + inner attrs) into the
// gen model, recursively, sorted by name.
func TestBlocksFromProviderSchema(t *testing.T) {
	sch := provider.ResourceSchema{
		TypeName: "aws_instance",
		Attrs:    []provider.Attr{{Name: "ami", Required: true}},
		Blocks: []provider.NestedBlock{
			{Name: "ingress", Nesting: provider.BlockList, Attrs: []provider.Attr{{Name: "from_port", Required: true}}},
			{Name: "root", Nesting: provider.BlockSingle, Blocks: []provider.NestedBlock{
				{Name: "inner", Nesting: provider.BlockSet, Attrs: []provider.Attr{{Name: "k", Optional: true}}},
			}},
		},
	}
	r := fromSchema(sch)
	if len(r.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2", len(r.Blocks))
	}
	// sorted by name: "ingress" before "root"
	if r.Blocks[0].Name != "ingress" || r.Blocks[0].Nesting != NestList {
		t.Errorf("block 0 = %+v, want ingress/list", r.Blocks[0])
	}
	if r.Blocks[1].Name != "root" || r.Blocks[1].Nesting != NestSingle {
		t.Errorf("block 1 = %+v, want root/single", r.Blocks[1])
	}
	// recursion: root has a sub-block "inner" (set-nested)
	if len(r.Blocks[1].Blocks) != 1 || r.Blocks[1].Blocks[0].Nesting != NestSet {
		t.Errorf("nested-in-nested block not mapped: %+v", r.Blocks[1].Blocks)
	}
}
