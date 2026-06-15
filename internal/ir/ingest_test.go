// Copyright 2026 WeareTechnative B.V. and the nixform authors
// SPDX-License-Identifier: Apache-2.0

package ir_test

import (
	"strings"
	"testing"

	"github.com/wearetechnative/nixform/internal/ir"
)

const validHeadline = `{
  "schemaVersion": 1,
  "providers": {
    "alpha": {"source": "./bin/provider-alpha", "config": {}},
    "beta":  {"source": "./bin/provider-beta",  "config": {}}
  },
  "resources": [
    {"id":"alpha.alpha_token.A","provider":"alpha","type":"alpha_token","name":"A","config":{"label":"seed"}},
    {"id":"beta.beta_record.B","provider":"beta","type":"beta_record","name":"B",
     "config":{"from":{"__derived":{"inputs":["alpha.alpha_token.A.value"]}}}},
    {"id":"alpha.alpha_token.C","provider":"alpha","type":"alpha_token","name":"C",
     "config":{"label":{"__ref":{"resource":"beta.beta_record.B","path":["endpoint"]}}}}
  ],
  "edges": [
    {"from":"alpha.alpha_token.A","to":"beta.beta_record.B","via":"from"},
    {"from":"beta.beta_record.B","to":"alpha.alpha_token.C","via":"label"}
  ],
  "nixConsumers": [
    {"id":"systemConfig","value":{
      "tokenValue":{"__ref":{"resource":"alpha.alpha_token.A","path":["value"]}},
      "recordEndpoint":{"__ref":{"resource":"beta.beta_record.B","path":["endpoint"]}}
    }}
  ]
}`

func TestIngestValid(t *testing.T) {
	g, err := ir.IngestIR([]byte(validHeadline))
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if len(g.Nodes) != 3 {
		t.Fatalf("nodes = %d, want 3", len(g.Nodes))
	}
	if got := g.Order; len(got) != 3 || got[0] != "alpha.alpha_token.A" {
		t.Fatalf("order = %v, want A first", got)
	}
}

func TestIngestClassification(t *testing.T) {
	g, err := ir.IngestIR([]byte(validHeadline))
	if err != nil {
		t.Fatal(err)
	}
	// C.config.label is a __ref -> TF->TF
	c := g.Nodes["alpha.alpha_token.C"]
	if len(c.Refs) != 1 || c.Refs[0].Class != ir.ClassTFTF || c.Refs[0].Target != "beta.beta_record.B" {
		t.Fatalf("C refs = %+v, want one TF->TF to B", c.Refs)
	}
	// B.config.from is a __derived -> *->Nix
	b := g.Nodes["beta.beta_record.B"]
	if len(b.Refs) != 1 || b.Refs[0].Class != ir.ClassStarToNix {
		t.Fatalf("B refs = %+v, want one *->Nix derived", b.Refs)
	}
	// systemConfig consumer refs are all *->Nix.
	var consumerRefs int
	for _, e := range g.AllRefs {
		if e.Owner == "systemConfig" {
			consumerRefs++
			if e.Class != ir.ClassStarToNix {
				t.Errorf("consumer ref %+v should be *->Nix", e)
			}
		}
	}
	if consumerRefs != 2 {
		t.Errorf("systemConfig refs = %d, want 2", consumerRefs)
	}
}

func TestIngestMalformed(t *testing.T) {
	cases := []struct {
		name    string
		ir      string
		wantSub string
	}{
		{
			name: "dangling edge",
			ir: `{"schemaVersion":1,"providers":{"a":{"source":"x","config":{}}},
				"resources":[{"id":"a.t.A","provider":"a","type":"t","name":"A","config":{}}],
				"edges":[{"from":"a.t.A","to":"a.t.ghost","via":"x"}],"nixConsumers":[]}`,
			wantSub: "a.t.ghost",
		},
		{
			name: "duplicate id",
			ir: `{"schemaVersion":1,"providers":{"a":{"source":"x","config":{}}},
				"resources":[{"id":"a.t.A","provider":"a","type":"t","name":"A","config":{}},
				             {"id":"a.t.A","provider":"a","type":"t","name":"A","config":{}}],
				"edges":[],"nixConsumers":[]}`,
			wantSub: "duplicate resource id \"a.t.A\"",
		},
		{
			name: "undeclared provider",
			ir: `{"schemaVersion":1,"providers":{"a":{"source":"x","config":{}}},
				"resources":[{"id":"ghost.t.A","provider":"ghost","type":"t","name":"A","config":{}}],
				"edges":[],"nixConsumers":[]}`,
			wantSub: "undeclared provider \"ghost\"",
		},
		{
			name: "ref to missing resource",
			ir: `{"schemaVersion":1,"providers":{"a":{"source":"x","config":{}}},
				"resources":[{"id":"a.t.A","provider":"a","type":"t","name":"A",
				  "config":{"x":{"__ref":{"resource":"a.t.missing","path":["v"]}}}}],
				"edges":[],"nixConsumers":[]}`,
			wantSub: "a.t.missing",
		},
		{
			name: "count not expanded (meta)",
			ir: `{"schemaVersion":1,"providers":{"a":{"source":"x","config":{}}},
				"resources":[{"id":"a.t.A","provider":"a","type":"t","name":"A","config":{},
				  "meta":{"count":3}}],
				"edges":[],"nixConsumers":[]}`,
			wantSub: "count",
		},
		{
			name: "malformed ref (no path)",
			ir: `{"schemaVersion":1,"providers":{"a":{"source":"x","config":{}}},
				"resources":[{"id":"a.t.A","provider":"a","type":"t","name":"A",
				  "config":{"x":{"__ref":{"resource":"a.t.A"}}}}],
				"edges":[],"nixConsumers":[]}`,
			wantSub: "path",
		},
		{
			name:    "bad schema version",
			ir:      `{"schemaVersion":2,"providers":{},"resources":[],"edges":[],"nixConsumers":[]}`,
			wantSub: "schemaVersion",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ir.IngestIR([]byte(tc.ir))
			if err == nil {
				t.Fatalf("expected error mentioning %q, got nil", tc.wantSub)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("error %q does not mention %q", err.Error(), tc.wantSub)
			}
		})
	}
}
