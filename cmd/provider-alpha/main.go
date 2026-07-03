// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Command provider-alpha is a fake tfprotov6 provider exposing the alpha_token
// resource. It is hermetic and deterministic (DESIGN D6); see docs/TESTING.md.
package main

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/nivis-project/nivis/internal/fakeprovider"
)

// AlphaToken is the alpha_token resource definition, exported so the in-process
// conformance test can construct the server without spawning a binary.
var AlphaToken = fakeprovider.Resource{
	TypeName: "alpha_token",
	Attrs: map[string]fakeprovider.Attr{
		"label": {Type: tftypes.String, Optional: true},
		"id":    {Type: tftypes.String, Computed: true},
		"value": {Type: tftypes.String, Computed: true},
	},
	Apply: func(inputs map[string]string, counter int64) (map[string]string, []*tfprotov6.Diagnostic) {
		label := inputs["label"] // "" when absent
		return map[string]string{
			"id":    fmt.Sprintf("alpha-%d", counter),
			"value": fmt.Sprintf("alpha:%s:%d", label, counter),
		}, nil
	},
}

// AlphaLookup is a datasource: it READS a value derived from its `query` input,
// modeling "look up existing infrastructure". Deterministic in the config.
var AlphaLookup = fakeprovider.Resource{
	TypeName: "alpha_lookup",
	Attrs: map[string]fakeprovider.Attr{
		"query":  {Type: tftypes.String, Required: true},
		"id":     {Type: tftypes.String, Computed: true},
		"result": {Type: tftypes.String, Computed: true},
	},
	Apply: func(inputs map[string]string, _ int64) (map[string]string, []*tfprotov6.Diagnostic) {
		q := inputs["query"]
		return map[string]string{
			"id":     fmt.Sprintf("lookup-%s", q),
			"result": fmt.Sprintf("found:%s", q),
		}, nil
	},
}

// NewServer builds the provider server (shared by main and the test).
func NewServer() *fakeprovider.Server {
	return fakeprovider.New(AlphaToken).WithDataSources(AlphaLookup)
}

func main() {
	err := tf6server.Serve(
		"registry.nivis.test/fake/alpha",
		func() tfprotov6.ProviderServer { return NewServer() },
	)
	if err != nil {
		log.Fatal(err)
	}
}
