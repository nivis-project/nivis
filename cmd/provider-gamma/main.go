// Command provider-gamma is a fake tfprotov5 provider exposing the gamma_widget
// resource. It is hermetic and deterministic (DESIGN D6); see docs/TESTING.md.
// It mirrors provider-alpha/provider-beta but speaks plugin protocol version 5.
package main

import (
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5/tf5server"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/wearetechnative/nixform/internal/fakeproviderv5"
)

// GammaWidget is the gamma_widget resource definition, exported so the
// in-process conformance test can construct the server without spawning a
// binary.
var GammaWidget = fakeproviderv5.Resource{
	TypeName: "gamma_widget",
	Attrs: map[string]fakeproviderv5.Attr{
		"size":   {Type: tftypes.String, Required: true},
		"id":     {Type: tftypes.String, Computed: true},
		"result": {Type: tftypes.String, Computed: true},
	},
	Apply: func(inputs map[string]string, counter int64) (map[string]string, []*tfprotov5.Diagnostic) {
		size := inputs["size"]
		return map[string]string{
			"id":     fmt.Sprintf("gamma-%d", counter),
			"result": fmt.Sprintf("widget:%s:%d", size, counter),
		}, nil
	},
}

// NewServer builds the provider server (shared by main and the test).
func NewServer() *fakeproviderv5.Server { return fakeproviderv5.New(GammaWidget) }

func main() {
	err := tf5server.Serve(
		"registry.nixform.test/fake/gamma",
		func() tfprotov5.ProviderServer { return NewServer() },
	)
	if err != nil {
		log.Fatal(err)
	}
}
