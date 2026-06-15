// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package v6

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	"github.com/wearetechnative/nivis/internal/tfplugin6"
)

// countingClient is a minimal tfplugin6.ProviderClient that records how many
// times GetProviderSchema is called and returns a fixed two-resource schema.
// Only the methods the cache test exercises are implemented; the rest panic if
// unexpectedly called.
type countingClient struct {
	tfplugin6.ProviderClient // embed for the methods we don't implement
	schemaCalls              int
}

func (c *countingClient) GetProviderSchema(_ context.Context, _ *tfplugin6.GetProviderSchema_Request, _ ...grpc.CallOption) (*tfplugin6.GetProviderSchema_Response, error) {
	c.schemaCalls++
	strType := []byte(`"string"`)
	mkRes := func() *tfplugin6.Schema {
		return &tfplugin6.Schema{Block: &tfplugin6.Schema_Block{
			Attributes: []*tfplugin6.Schema_Attribute{
				{Name: "name", Type: strType, Required: true},
				{Name: "id", Type: strType, Computed: true},
			},
		}}
	}
	return &tfplugin6.GetProviderSchema_Response{
		ResourceSchemas: map[string]*tfplugin6.Schema{
			"x_one": mkRes(),
			"x_two": mkRes(),
		},
	}, nil
}

func TestSchemaFetchedOnce(t *testing.T) {
	cc := &countingClient{}
	b := New(cc)
	ctx := context.Background()

	if _, err := b.ListResourceTypes(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := b.GetSchema(ctx, "x_one"); err != nil {
			t.Fatal(err)
		}
		if _, err := b.GetSchema(ctx, "x_two"); err != nil {
			t.Fatal(err)
		}
	}
	if cc.schemaCalls != 1 {
		t.Fatalf("GetProviderSchema called %d times, want 1 (schema must be cached)", cc.schemaCalls)
	}
}

func TestCachedSchemaReturnsCorrectData(t *testing.T) {
	b := New(&countingClient{})
	types, err := b.ListResourceTypes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(types) != 2 || types[0] != "x_one" || types[1] != "x_two" {
		t.Fatalf("types = %v, want [x_one x_two]", types)
	}
	rs, err := b.GetSchema(context.Background(), "x_one")
	if err != nil {
		t.Fatal(err)
	}
	if rs.TypeName != "x_one" || len(rs.Attrs) != 2 {
		t.Fatalf("schema = %+v, want x_one with 2 attrs", rs)
	}
}
