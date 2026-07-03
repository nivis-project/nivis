// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package tfvalue

import (
	"reflect"
	"testing"

	"github.com/nivis-project/nivis/internal/tfplugin6"
)

// TestProviderConfigRoundTrip proves that a provider config map declared in Nix
// — including a *nested block* — is faithfully serialized into the value the
// executor sends to ConfigureProvider. It mirrors the v6 backend's Configure
// path: build the provider-config ObjectType from the provider schema block,
// EncodeConfig the map, then DecodeState it back and compare.
//
// The schema mirrors the relevant shape of the real hashicorp/aws provider used
// by nix/example/aws.nix: a scalar `region` attribute and a single-nested
// `default_tags` block carrying a `tags = map(string)` attribute (beans-prj4).
func TestProviderConfigRoundTrip(t *testing.T) {
	mapStringType := []byte(`["map","string"]`)

	block := &tfplugin6.Schema_Block{
		Version: 1,
		Attributes: []*tfplugin6.Schema_Attribute{
			{Name: "region", Type: []byte(`"string"`), Optional: true},
		},
		BlockTypes: []*tfplugin6.Schema_NestedBlock{
			{
				TypeName: "default_tags",
				Nesting:  tfplugin6.Schema_NestedBlock_SINGLE,
				Block: &tfplugin6.Schema_Block{
					Version: 1,
					Attributes: []*tfplugin6.Schema_Attribute{
						{Name: "tags", Type: mapStringType, Optional: true},
					},
				},
			},
		},
	}

	objType, err := ObjectType(block)
	if err != nil {
		t.Fatalf("ObjectType: %v", err)
	}

	// The config map as it arrives from the IR (mkProvider's config, resolved).
	config := map[string]interface{}{
		"region": "eu-central-1",
		"default_tags": map[string]interface{}{
			"tags": map[string]interface{}{"managed-by": "nivis"},
		},
	}

	dv, err := EncodeConfig(objType, map[string]bool{}, config)
	if err != nil {
		t.Fatalf("EncodeConfig: %v", err)
	}

	got, err := DecodeState(objType, dv)
	if err != nil {
		t.Fatalf("DecodeState: %v", err)
	}

	if got["region"] != "eu-central-1" {
		t.Errorf("region: got %#v, want %q", got["region"], "eu-central-1")
	}

	// The nested block must survive the round trip with its inner map intact.
	wantBlock := map[string]interface{}{
		"tags": map[string]interface{}{"managed-by": "nivis"},
	}
	if !reflect.DeepEqual(got["default_tags"], wantBlock) {
		t.Errorf("default_tags nested block: got %#v, want %#v", got["default_tags"], wantBlock)
	}
}

// TestProviderConfigOmittedBecomesNull confirms attributes present in the schema
// but absent from the Nix config encode without error (they become null) — so a
// partial provider config is valid, not a hard failure.
func TestProviderConfigOmittedBecomesNull(t *testing.T) {
	block := &tfplugin6.Schema_Block{
		Version: 1,
		Attributes: []*tfplugin6.Schema_Attribute{
			{Name: "region", Type: []byte(`"string"`), Optional: true},
			{Name: "profile", Type: []byte(`"string"`), Optional: true},
		},
	}
	objType, err := ObjectType(block)
	if err != nil {
		t.Fatalf("ObjectType: %v", err)
	}

	// Only region set; profile omitted.
	dv, err := EncodeConfig(objType, map[string]bool{}, map[string]interface{}{"region": "eu-west-1"})
	if err != nil {
		t.Fatalf("EncodeConfig with omitted attr: %v", err)
	}
	got, err := DecodeState(objType, dv)
	if err != nil {
		t.Fatalf("DecodeState: %v", err)
	}
	if got["region"] != "eu-west-1" {
		t.Errorf("region: got %#v, want %q", got["region"], "eu-west-1")
	}
	// The omitted attribute must not error and must not carry a spurious value:
	// it decodes as null (Go nil), so the provider applies its own default.
	if v := got["profile"]; v != nil {
		t.Errorf("omitted profile should decode as null, got %#v", v)
	}
}
