// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Command provider-zeta is a fake tfprotov6 provider that EMITS PROVIDER LOG
// LINES while planning, so the executor's provider-note rendering is provable
// hermetically (no network, no real provider, no credentials). The other fakes
// are silent and so cannot catch a rendering regression.
//
// It emits on both routes the plugin transport distinguishes, because they reach
// the renderer by different paths:
//
//   - a STRUCTURED entry (JSON), which go-plugin parses and re-emits at the
//     level it declares with every field intact. This one reproduces the
//     real-world entry from bean nixform2-ceoh verbatim — including an `error`
//     field on a WARN entry, the case that makes a benign note read as a failure.
//   - a PREFIXED PLAIN-TEXT line, whose level go-plugin can only infer from its
//     "[WARN]" prefix and which carries no fields at all.
//
// Emission is deterministic and needs no environment variable: it happens once
// per PlanResourceChange, with fixed ids and a fixed timestamp, so a test can
// assert on exact output. It is hermetic and deterministic (DESIGN D6); see
// docs/TESTING.md.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6/tf6server"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/nivis-project/nivis/internal/fakeprovider"
)

// realStderr is the process's stderr as it exists BEFORE go-plugin's serve
// replaces os.Stderr with the plugin's stdio channel.
//
// This distinction is the whole reason a fake can get this wrong: writes to
// os.Stderr AFTER serving are captured by that channel and delivered to the
// client's SyncStderr (io.Discard by default), so they vanish. The transport's
// log reader reads the process's actual stderr descriptor instead — which is
// where a real provider's logging lands, because a real provider builds its
// logger at init from the original descriptor, exactly as this package-level
// variable does.
var realStderr = os.Stderr

// ZetaNote is an ordinary resource: the logging is the point, not the resource.
var ZetaNote = fakeprovider.Resource{
	TypeName: "zeta_note",
	Attrs: map[string]fakeprovider.Attr{
		"label": {Type: tftypes.String, Optional: true},
		"id":    {Type: tftypes.String, Computed: true},
		"value": {Type: tftypes.String, Computed: true},
	},
	Apply: func(inputs map[string]string, counter int64) (map[string]string, []*tfprotov6.Diagnostic) {
		label := inputs["label"]
		return map[string]string{
			"id":    fmt.Sprintf("zeta-%d", counter),
			"value": fmt.Sprintf("zeta:%s:%d", label, counter),
		}, nil
	},
}

// structuredEntry is the real-world entry from the bug report, parameterised only
// by the resource type being planned. Everything else is fixed so the output is
// byte-stable across runs.
//
// The timestamp must match the layout go-plugin's JSON parser expects
// (2006-01-02T15:04:05.000000Z07:00); a malformed one would make the whole line
// fall through to the plain-text route and silently test the wrong path.
func structuredEntry(typeName string) string {
	return `{"@level":"warn",` +
		`"@message":"unable to require attribute replacement",` +
		`"@module":"sdk.helper_schema",` +
		`"@timestamp":"2026-08-31T23:58:27.974000+02:00",` +
		`"@caller":"/opt/provider/helper/customdiff/force_new.go:66",` +
		`"error":"ForceNew: No changes for description",` +
		`"tf_attribute_path":"description",` +
		`"tf_mux_provider":"*schema.GRPCProviderServer",` +
		`"tf_provider_addr":"registry.terraform.io/nivis/zeta",` +
		`"tf_resource_type":"` + typeName + `",` +
		`"tf_rpc":"PlanResourceChange",` +
		`"tf_req_id":"dd296e15-1f2a-4c7b-9d3e-6a1b2c3d4e5f"}`
}

// plainEntry is the other route: a level discoverable only from the prefix, with
// no fields to derive a subject from.
const plainEntry = `[WARN] planning with a legacy code path`

func main() {
	server := fakeprovider.New(ZetaNote).WithPlanLogEmitter(func(typeName string) {
		// The process's real stderr, which the plugin transport reads line by
		// line — NOT os.Stderr (by now the plugin's stdio channel, which is
		// discarded) and NOT stdout, which carries the gRPC handshake.
		fmt.Fprintln(realStderr, structuredEntry(typeName))
		fmt.Fprintln(realStderr, plainEntry)
	})

	err := tf6server.Serve(
		"registry.terraform.io/nivis/zeta",
		func() tfprotov6.ProviderServer { return server },
	)
	if err != nil {
		log.Fatal(err)
	}
}
