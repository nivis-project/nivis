// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase_test

import (
	"context"
	"testing"

	"github.com/nivis-project/nivis/internal/ledger"
	"github.com/nivis-project/nivis/internal/phase"
	"github.com/nivis-project/nivis/internal/plugin"
	"github.com/nivis-project/nivis/internal/state"
)

// ResolveOutputs collects the reserved output.<name> consumers and unwraps each
// { value } to the resolved value, keyed by the name after the prefix. A
// non-output consumer is ignored.
func TestResolveOutputsUnwraps(t *testing.T) {
	ir := func(_ *ledger.Ledger) []byte {
		return []byte(`{
		  "schemaVersion":1,
		  "providers":{},
		  "resources":[],
		  "edges":[],
		  "nixConsumers":[
		    {"id":"output.url","value":{"value":"http://1.2.3.4"}},
		    {"id":"output.count","value":{"value":3}},
		    {"id":"systemConfig","value":{"x":"ignored"}}
		  ]
		}`)
	}
	mgr := plugin.NewManager()
	defer mgr.Close()
	st, _ := state.Open(t.TempDir() + "/state.json")
	d := &phase.Driver{Eval: &phase.StubEvaluator{IRForLedger: ir}, Manager: mgr, Store: st, Ledger: ledger.New()}

	outs, err := d.ResolveOutputs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if outs["url"] != "http://1.2.3.4" {
		t.Errorf("url = %#v, want the unwrapped string", outs["url"])
	}
	if outs["count"] != float64(3) { // JSON number
		t.Errorf("count = %#v, want 3", outs["count"])
	}
	if _, ok := outs["systemConfig"]; ok {
		t.Error("a non-output consumer must not appear in outputs")
	}
	if len(outs) != 2 {
		t.Errorf("outputs = %v, want exactly url + count", outs)
	}
}
