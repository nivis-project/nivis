// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package v6

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	"github.com/wearetechnative/nivis/internal/provider"
	"github.com/wearetechnative/nivis/internal/tfplugin6"
)

// blockClient returns a schema with a list-nested and a single-nested block, so
// the backend's GetSchema can be checked for surfacing nested blocks.
type blockClient struct {
	tfplugin6.ProviderClient
}

func (c *blockClient) GetProviderSchema(_ context.Context, _ *tfplugin6.GetProviderSchema_Request, _ ...grpc.CallOption) (*tfplugin6.GetProviderSchema_Response, error) {
	strType := []byte(`"string"`)
	return &tfplugin6.GetProviderSchema_Response{
		ResourceSchemas: map[string]*tfplugin6.Schema{
			"x_res": {Block: &tfplugin6.Schema_Block{
				Attributes: []*tfplugin6.Schema_Attribute{
					{Name: "name", Type: strType, Required: true},
				},
				BlockTypes: []*tfplugin6.Schema_NestedBlock{
					{
						TypeName: "ingress",
						Nesting:  tfplugin6.Schema_NestedBlock_LIST,
						Block: &tfplugin6.Schema_Block{Attributes: []*tfplugin6.Schema_Attribute{
							{Name: "from_port", Type: strType, Required: true},
						}},
					},
					{
						TypeName: "settings",
						Nesting:  tfplugin6.Schema_NestedBlock_SINGLE,
						Block: &tfplugin6.Schema_Block{Attributes: []*tfplugin6.Schema_Attribute{
							{Name: "mode", Type: strType, Optional: true},
						}},
					},
				},
			}},
		},
	}, nil
}

func TestGetSchemaSurfacesNestedBlocks(t *testing.T) {
	b := New(&blockClient{})
	rs, err := b.GetSchema(context.Background(), "x_res")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Blocks) != 2 {
		t.Fatalf("blocks = %d, want 2: %+v", len(rs.Blocks), rs.Blocks)
	}
	byName := map[string]provider.NestedBlock{}
	for _, blk := range rs.Blocks {
		byName[blk.Name] = blk
	}
	if byName["ingress"].Nesting != provider.BlockList {
		t.Errorf("ingress nesting = %q, want list", byName["ingress"].Nesting)
	}
	if len(byName["ingress"].Attrs) != 1 || byName["ingress"].Attrs[0].Name != "from_port" {
		t.Errorf("ingress inner attrs = %+v, want [from_port]", byName["ingress"].Attrs)
	}
	if byName["settings"].Nesting != provider.BlockSingle {
		t.Errorf("settings nesting = %q, want single", byName["settings"].Nesting)
	}
}
