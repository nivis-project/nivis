// Copyright 2026 WeareTechnative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package v5

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestV5EncodeDecodeCollections confirms the v5 codec wrappers
// (encodeState/decodeState over the tfplugin5 DynamicValue) round-trip
// collection and object attributes, not just scalars.
func TestV5EncodeDecodeCollections(t *testing.T) {
	objType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{
		"name":  tftypes.String,
		"tags":  tftypes.Map{ElementType: tftypes.String},
		"ports": tftypes.List{ElementType: tftypes.Number},
	}}
	in := map[string]interface{}{
		"name":  "demo",
		"tags":  map[string]interface{}{"env": "prod"},
		"ports": []interface{}{float64(80), float64(443)},
	}

	dv, err := encodeState(objType, in)
	if err != nil {
		t.Fatalf("encodeState: %v", err)
	}
	out, err := decodeState(objType, dv)
	if err != nil {
		t.Fatalf("decodeState: %v", err)
	}
	if out["name"] != "demo" {
		t.Errorf("name = %v", out["name"])
	}
	if !reflect.DeepEqual(out["tags"], map[string]interface{}{"env": "prod"}) {
		t.Errorf("tags = %#v", out["tags"])
	}
	if !reflect.DeepEqual(out["ports"], []interface{}{float64(80), float64(443)}) {
		t.Errorf("ports = %#v", out["ports"])
	}
}
