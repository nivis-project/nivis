// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package ir_test

import (
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/ir"
)

// minimal valid IR with a substitutable backend snippet. %s is the (optional)
// `"backend": {...},` fragment.
func irWithBackend(backendFragment string) string {
	return `{
	  "schemaVersion": 1,
	  ` + backendFragment + `
	  "providers": {"alpha": {"source": "p", "config": {}}},
	  "resources": [
	    {"id":"alpha.alpha_token.A","provider":"alpha","type":"alpha_token","name":"A","config":{}}
	  ],
	  "edges": [],
	  "nixConsumers": []
	}`
}

// A valid, static s3-shaped backend parses and is available on the Graph.
func TestIngestBackendValid(t *testing.T) {
	g, err := ir.IngestIR([]byte(irWithBackend(`"backend": {"type":"s3","bucket":"b","key":"prod/app","region":"eu-west-1"},`)))
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if g.Backend == nil {
		t.Fatal("Graph.Backend is nil, want the parsed backend")
	}
	if g.Backend["type"] != "s3" || g.Backend["bucket"] != "b" || g.Backend["key"] != "prod/app" {
		t.Errorf("backend = %+v, want s3/b/prod/app", g.Backend)
	}
}

// No backend => nil Backend, the local file store (unchanged behaviour).
func TestIngestNoBackend(t *testing.T) {
	g, err := ir.IngestIR([]byte(irWithBackend("")))
	if err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if g.Backend != nil {
		t.Errorf("Backend = %+v, want nil when no backend declared", g.Backend)
	}
}

// Backend validation rejects: missing type, empty type, and a ref leaf (which
// would mean the location is not statically known).
func TestIngestBackendRejected(t *testing.T) {
	cases := []struct {
		name     string
		fragment string
		wantSub  string
	}{
		{
			name:     "missing type",
			fragment: `"backend": {"bucket":"b"},`,
			wantSub:  "backend.type is required",
		},
		{
			name:     "empty type",
			fragment: `"backend": {"type":""},`,
			wantSub:  "backend.type must be a non-empty string",
		},
		{
			name:     "ref leaf",
			fragment: `"backend": {"type":"s3","bucket":{"__ref":{"resource":"alpha.alpha_token.A","path":["id"]}}},`,
			wantSub:  "must be static",
		},
		{
			name:     "derived leaf nested",
			fragment: `"backend": {"type":"s3","opts":{"key":{"__derived":{"inputs":["x"]}}}},`,
			wantSub:  "must be static",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ir.IngestIR([]byte(irWithBackend(tc.fragment)))
			if err == nil {
				t.Fatalf("expected an error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}

// The ref-leaf rejection names the offending path, so the user can find it.
func TestIngestBackendRefNamesPath(t *testing.T) {
	_, err := ir.IngestIR([]byte(irWithBackend(`"backend": {"type":"s3","bucket":{"__ref":{"resource":"alpha.alpha_token.A","path":["id"]}}},`)))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "backend.bucket") {
		t.Errorf("error %q should name the path backend.bucket", err.Error())
	}
}
