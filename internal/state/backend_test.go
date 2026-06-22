// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/wearetechnative/nivis/internal/state"
)

// OpenBackend selects the right store from an IR backend block and errors clearly
// on an unsupported type or a missing required s3 key.
func TestOpenBackendSelection(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	local := filepath.Join(t.TempDir(), "state.json")

	// nil backend => local file store, usable.
	st, err := state.OpenBackend(nil, local)
	if err != nil {
		t.Fatalf("nil backend: %v", err)
	}
	if err := st.Set(state.ResourceState{ID: "a.b.c", Type: "t", Attrs: map[string]interface{}{}}); err != nil {
		t.Fatalf("local store unusable: %v", err)
	}

	// explicit local type => local store.
	if _, err := state.OpenBackend(map[string]interface{}{"type": "local"}, local); err != nil {
		t.Fatalf("local type: %v", err)
	}

	// s3 with all required keys => an s3 store is constructed (no call made yet).
	if _, err := state.OpenBackend(map[string]interface{}{
		"type": "s3", "bucket": "b", "key": "k", "region": "r",
	}, local); err != nil {
		t.Fatalf("valid s3 backend should open: %v", err)
	}
}

func TestOpenBackendErrors(t *testing.T) {
	cases := []struct {
		name    string
		backend map[string]interface{}
		wantSub string
	}{
		{"unsupported type", map[string]interface{}{"type": "gcs"}, "unsupported backend type"},
		{"s3 missing bucket", map[string]interface{}{"type": "s3", "key": "k", "region": "r"}, `requires "bucket"`},
		{"s3 missing key", map[string]interface{}{"type": "s3", "bucket": "b", "region": "r"}, `requires "key"`},
		{"s3 missing region", map[string]interface{}{"type": "s3", "bucket": "b", "key": "k"}, `requires "region"`},
		{"s3 empty bucket", map[string]interface{}{"type": "s3", "bucket": "", "key": "k", "region": "r"}, "non-empty string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := state.OpenBackend(tc.backend, "")
			if err == nil {
				t.Fatalf("expected an error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %q, want it to contain %q", err.Error(), tc.wantSub)
			}
		})
	}
}
