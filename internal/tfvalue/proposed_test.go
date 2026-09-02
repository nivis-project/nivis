// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package tfvalue

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// TestProposedMsgPack_MergesPriorIntoUnsetComputed covers the objchange
// contract that fixes the perpetual -/+ on unchanged resources (bean
// nixform2-tqkd): computed attributes absent from config must carry the PRIOR
// value (never unknown against an existing resource), plain optional attributes
// absent from config must be null, and config values win where present.
func TestProposedMsgPack_MergesPriorIntoUnsetComputed(t *testing.T) {
	objType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"name":        tftypes.String, // required, in config
			"name_prefix": tftypes.String, // optional+computed+ForceNew style
			"id":          tftypes.String, // pure computed
			"desc":        tftypes.String, // plain optional, being unset
			"ref_attr":    tftypes.String, // unresolved ref in config
			"never_set":   tftypes.String, // computed, absent from prior too
		},
	}
	computed := map[string]bool{"name_prefix": true, "id": true, "never_set": true}
	config := map[string]interface{}{
		"name":     "web",
		"ref_attr": map[string]interface{}{"__ref": map[string]interface{}{"resource": "a.b.c", "path": []interface{}{"id"}}},
	}
	prior := map[string]interface{}{
		"name":        "web",
		"name_prefix": "", // the aws_iam_role.name_prefix case
		"id":          "AROA123",
		"desc":        "old description",
	}

	mp, err := ProposedMsgPack(objType, computed, config, prior)
	if err != nil {
		t.Fatalf("ProposedMsgPack: %v", err)
	}
	val, err := tftypes.ValueFromMsgPack(mp, objType) //nolint:staticcheck // wire format
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	var attrs map[string]tftypes.Value
	if err := val.As(&attrs); err != nil {
		t.Fatalf("as map: %v", err)
	}

	assertKnownString := func(name, want string) {
		t.Helper()
		v := attrs[name]
		if !v.IsKnown() {
			t.Fatalf("%s: unknown, want known %q", name, want)
		}
		var got string
		if err := v.As(&got); err != nil {
			t.Fatalf("%s: as string: %v", name, err)
		}
		if got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}

	assertKnownString("name", "web")        // config wins
	assertKnownString("name_prefix", "")    // MERGED from prior — the fix
	assertKnownString("id", "AROA123")      // pure computed carries prior
	if attrs["desc"].IsKnown() == false || !attrs["desc"].IsNull() {
		t.Fatalf("desc = %v, want known null (unsetting an optional attr plans a removal)", attrs["desc"])
	}
	if attrs["ref_attr"].IsKnown() {
		t.Fatalf("ref_attr = %v, want unknown (unresolved ref)", attrs["ref_attr"])
	}
	if !attrs["never_set"].IsNull() {
		t.Fatalf("never_set = %v, want null (computed, no prior value)", attrs["never_set"])
	}
}
