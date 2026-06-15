// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
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

	"github.com/wearetechnative/terrae-nivis/internal/fakeprovider"
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

// NewServer builds the provider server (shared by main and the test).
func NewServer() *fakeprovider.Server { return fakeprovider.New(AlphaToken) }

func main() {
	err := tf6server.Serve(
		"registry.terrae-nivis.test/fake/alpha",
		func() tfprotov6.ProviderServer { return NewServer() },
	)
	if err != nil {
		log.Fatal(err)
	}
}
