// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Command provider-delta is a fake tfprotov6 provider whose resource uses
// collection and nested-object attributes, exercising the value codec
// end-to-end. Hermetic and deterministic; see docs/TESTING.md.
package main

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/nivis-project/nivis/internal/fakeproviderx"
)

// metaType is the nested-object output type: object({ region=string, count=number }).
var metaType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
	"region": tftypes.String,
	"count":  tftypes.Number,
}}

// DeltaThing exercises map(string)/list(number) inputs and
// list(string)/object computed outputs.
var DeltaThing = fakeproviderx.Resource{
	TypeName: "delta_thing",
	Attrs: map[string]fakeproviderx.Attr{
		"label":     {Type: tftypes.String, Optional: true},
		"tags":      {Type: tftypes.Map{ElementType: tftypes.String}, Optional: true},
		"ports":     {Type: tftypes.List{ElementType: tftypes.Number}, Optional: true},
		"id":        {Type: tftypes.String, Computed: true},
		"endpoints": {Type: tftypes.List{ElementType: tftypes.String}, Computed: true},
		"meta":      {Type: metaType, Computed: true},
	},
	Apply: func(inputs map[string]interface{}, counter int64) map[string]interface{} {
		// endpoints: one "ep-<port>-<counter>" per input port (deterministic).
		var endpoints []interface{}
		if ports, ok := inputs["ports"].([]interface{}); ok {
			for _, p := range ports {
				port, _ := p.(float64)
				endpoints = append(endpoints, fmt.Sprintf("ep-%d-%d", int(port), counter))
			}
		}
		if endpoints == nil {
			endpoints = []interface{}{}
		}
		// meta.region from tags["env"] (or "none"); meta.count = number of tags.
		region := "none"
		count := 0.0
		if tags, ok := inputs["tags"].(map[string]interface{}); ok {
			count = float64(len(tags))
			if env, ok := tags["env"].(string); ok {
				region = env
			}
		}
		return map[string]interface{}{
			"id":        fmt.Sprintf("delta-%d", counter),
			"endpoints": endpoints,
			"meta":      map[string]interface{}{"region": region, "count": count},
		}
	},
}

func NewServer() *fakeproviderx.Server { return fakeproviderx.New(DeltaThing) }

func main() {
	err := tf6server.Serve(
		"registry.nivis.test/fake/delta",
		func() tfprotov6.ProviderServer { return NewServer() },
	)
	if err != nil {
		log.Fatal(err)
	}
}
