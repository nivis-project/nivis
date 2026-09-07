// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state

// The destination guard, tested as the pure function it is: every row of the
// decision table, plus the forced variants. In-package because the guard is
// internal; the observable behaviour it produces is covered from outside too.

import (
	"errors"
	"strings"
	"testing"
)

// doc builds canonical document bytes holding the given resource ids.
func doc(t *testing.T, ids ...string) []byte {
	t.Helper()
	d := document{Resources: map[string]ResourceState{}}
	for _, id := range ids {
		d.Resources[id] = ResourceState{ID: id, Type: "alpha_token", Attrs: map[string]interface{}{"id": id}}
	}
	data, err := marshalDocument(d)
	if err != nil {
		t.Fatalf("marshalDocument: %v", err)
	}
	return data
}

// parseable wraps document bytes as a destination the guard can read.
func parseable(t *testing.T, data []byte) destinationState {
	t.Helper()
	n, err := countDocumentResources(data)
	if err != nil {
		t.Fatalf("countDocumentResources: %v", err)
	}
	return destinationState{Parseable: true, Snapshot: data, Resources: n}
}

func TestEvaluateGuardTable(t *testing.T) {
	src := doc(t, "a", "b")

	cases := []struct {
		name             string
		dst              destinationState
		force            bool
		wantVerdict      migrateVerdict
		wantConflct      bool
		wantUnparse      bool
		wantDstResources int
	}{
		{
			name:        "absent or empty destination proceeds",
			dst:         parseable(t, doc(t)),
			wantVerdict: verdictWrite,
		},
		{
			name:        "identical destination resumes",
			dst:         parseable(t, doc(t, "a", "b")),
			wantVerdict: verdictResume,
		},
		{
			name:             "differing resources are refused",
			dst:              parseable(t, doc(t, "c")),
			wantConflct:      true,
			wantDstResources: 1,
		},
		{
			name:        "differing resources are overwritten when forced",
			dst:         parseable(t, doc(t, "c")),
			force:       true,
			wantVerdict: verdictWrite,
		},
		{
			name:        "unparseable content is refused",
			dst:         destinationState{Parseable: false},
			wantConflct: true,
			wantUnparse: true,
		},
		{
			name:        "unparseable content is overwritten when forced",
			dst:         destinationState{Parseable: false},
			force:       true,
			wantVerdict: verdictWrite,
		},
		{
			// A superset is still "different": more resources than the source is not
			// a copy of it, so it must not be silently destroyed.
			name:             "a superset of the source is refused",
			dst:              parseable(t, doc(t, "a", "b", "c")),
			wantConflct:      true,
			wantDstResources: 3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			verdict, err := evaluateGuard(src, tc.dst, tc.force)
			if tc.wantConflct {
				var ce *ConflictError
				if !errors.As(err, &ce) {
					t.Fatalf("evaluateGuard error = %v (%T), want a *ConflictError", err, err)
				}
				if ce.Unparseable != tc.wantUnparse {
					t.Errorf("ConflictError.Unparseable = %v, want %v", ce.Unparseable, tc.wantUnparse)
				}
				if ce.SourceResources != 2 {
					t.Errorf("ConflictError.SourceResources = %d, want 2", ce.SourceResources)
				}
				if !tc.wantUnparse && ce.DestinationResources != tc.wantDstResources {
					t.Errorf("ConflictError.DestinationResources = %d, want %d", ce.DestinationResources, tc.wantDstResources)
				}
				return
			}
			if err != nil {
				t.Fatalf("evaluateGuard: unexpected error: %v", err)
			}
			if verdict != tc.wantVerdict {
				t.Errorf("verdict = %v, want %v", verdict, tc.wantVerdict)
			}
		})
	}
}

// An empty source into an empty destination is a legal no-op migration (it still
// removes the source), not a conflict. The empty-destination row wins, so it is a
// write of an empty document rather than a resume.
func TestEvaluateGuardEmptySource(t *testing.T) {
	verdict, err := evaluateGuard(doc(t), parseable(t, doc(t)), false)
	if err != nil {
		t.Fatalf("evaluateGuard: %v", err)
	}
	if verdict != verdictWrite {
		t.Errorf("verdict = %v, want verdictWrite", verdict)
	}
}

// The refusal messages carry the counts a caller reports to the user.
func TestConflictErrorMessages(t *testing.T) {
	ce := &ConflictError{SourceResources: 2, DestinationResources: 5}
	if msg := ce.Error(); !strings.Contains(msg, "5 resource(s)") ||
		!strings.Contains(msg, "source holds 2") || !strings.Contains(msg, "--force") {
		t.Errorf("conflict message = %q", msg)
	}
	un := &ConflictError{SourceResources: 2, Unparseable: true}
	if msg := un.Error(); !strings.Contains(msg, "not a Nivis state document") || !strings.Contains(msg, "--force") {
		t.Errorf("unparseable conflict message = %q", msg)
	}
}
