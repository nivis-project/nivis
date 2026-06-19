// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Command provider-epsilon is a fake tfprotov6 provider that REJECTS an
// unconfigured (all-null) ConfigureProvider, mimicking a credential-requiring
// real provider (proxmox/azurerm/google). It still serves GetProviderSchema
// normally, so it proves that `nivis gen` fetches the schema without configuring
// (the configure-rejecting case the always-succeeds fakes cannot catch). Hermetic
// and deterministic; see docs/TESTING.md.
package main

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wearetechnative/nivis/internal/fakeprovider"
)

// EpsilonThing is one resource type, so codegen has a schema to extract.
var EpsilonThing = fakeprovider.Resource{
	TypeName: "epsilon_thing",
	Attrs: map[string]fakeprovider.Attr{
		"name":  {Type: tftypes.String, Required: true},
		"id":    {Type: tftypes.String, Computed: true},
		"value": {Type: tftypes.String, Computed: true},
	},
	Apply: func(inputs map[string]string, counter int64) (map[string]string, []*tfprotov6.Diagnostic) {
		return map[string]string{
			"id":    fmt.Sprintf("epsilon-%d", counter),
			"value": fmt.Sprintf("epsilon:%s:%d", inputs["name"], counter),
		}, nil
	},
}

// NewServer builds the provider server. WithRequireConfigure makes its
// ConfigureProvider reject an all-null config (the point of this fake).
func NewServer() *fakeprovider.Server {
	return fakeprovider.New(EpsilonThing).WithRequireConfigure()
}

func main() {
	err := tf6server.Serve(
		"registry.nivis.test/fake/epsilon",
		func() tfprotov6.ProviderServer { return NewServer() },
	)
	if err != nil {
		log.Fatal(err)
	}
}
