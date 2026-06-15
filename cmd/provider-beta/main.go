// Command provider-beta is a fake tfprotov6 provider exposing the beta_record
// resource. It is hermetic and deterministic (DESIGN D6); see docs/TESTING.md.
package main

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wearetechnative/nixform/internal/fakeprovider"
)

// BetaRecord is the beta_record resource definition, exported for the
// in-process conformance test.
var BetaRecord = fakeprovider.Resource{
	TypeName: "beta_record",
	Attrs: map[string]fakeprovider.Attr{
		"from":     {Type: tftypes.String, Required: true},
		"endpoint": {Type: tftypes.String, Computed: true},
	},
	Apply: func(inputs map[string]string, _ int64) (map[string]string, []*tfprotov6.Diagnostic) {
		return map[string]string{
			"endpoint": fmt.Sprintf("beta://%s", inputs["from"]),
		}, nil
	},
}

// NewServer builds the provider server (shared by main and the test).
func NewServer() *fakeprovider.Server { return fakeprovider.New(BetaRecord) }

func main() {
	err := tf6server.Serve(
		"registry.nixform.test/fake/beta",
		func() tfprotov6.ProviderServer { return NewServer() },
	)
	if err != nil {
		log.Fatal(err)
	}
}
