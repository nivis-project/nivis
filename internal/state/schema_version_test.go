// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/state"
)

// A document carrying schemaVersion must restore. The field is new, and the
// whole-document seam is what `state pull` and `push` use.
func TestDocumentWithSchemaVersionParses(t *testing.T) {
	doc := []byte(`{"resources":{"a.b.c":{"id":"a.b.c","type":"b","attrs":{"x":"1"},"schemaVersion":3}}}`)
	st, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Restore(doc); err != nil {
		t.Fatalf("a document carrying schemaVersion must restore: %v", err)
	}
	rs, found, _ := st.Get("a.b.c")
	if !found || rs.SchemaVersion != 3 {
		t.Errorf("schemaVersion = %d, found = %v, want 3 and true", rs.SchemaVersion, found)
	}
}

// A document written BEFORE the field existed round-trips unchanged: no version
// appears, because the field is omitempty. That is what keeps an older nivis
// able to read what this one writes for resources it has not re-applied.
func TestDocumentWithoutSchemaVersionRoundTrips(t *testing.T) {
	doc := []byte(`{"resources":{"a.b.c":{"id":"a.b.c","type":"b","attrs":{"x":"1"}}}}`)
	st, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Restore(doc); err != nil {
		t.Fatal(err)
	}
	out, err := st.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "schemaVersion") {
		t.Errorf("a document with no recorded version gained one on round trip:\n%s", out)
	}
	rs, found, _ := st.Get("a.b.c")
	if !found || rs.SchemaVersion != 0 {
		t.Errorf("SchemaVersion = %d, want 0 (absent), which the executor reads as current", rs.SchemaVersion)
	}
}

// An unknown field does not make a document unreadable, which is what lets an
// OLDER nivis read a document this one wrote.
func TestUnknownFieldDoesNotRejectADocument(t *testing.T) {
	doc := []byte(`{"resources":{"a.b.c":{"id":"a.b.c","type":"b","attrs":{"x":"1"},"somethingNewer":42}}}`)
	st, err := state.Open(t.TempDir() + "/state.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Restore(doc); err != nil {
		t.Fatalf("an unknown field must be ignored, not rejected: %v", err)
	}
}
