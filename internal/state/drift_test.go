// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state_test

import (
	"testing"

	"github.com/nivis-project/nivis/internal/state"
)

func TestDrifted(t *testing.T) {
	cases := []struct {
		name              string
		stored, refreshed map[string]interface{}
		want              bool
	}{{
		name:      "identical",
		stored:    map[string]interface{}{"id": "a", "value": "v"},
		refreshed: map[string]interface{}{"id": "a", "value": "v"},
		want:      false,
	}, {
		// The case that matters: a provider read decodes at the full schema, so
		// it returns attributes the stored state never had, as nil. This is a
		// converged resource and must NOT be reported as drifted.
		name:      "refreshed carries extra nil schema attributes",
		stored:    map[string]interface{}{"id": "alpha-0", "value": "alpha::0"},
		refreshed: map[string]interface{}{"id": "alpha-0", "value": "alpha::0", "label": nil},
		want:      false,
	}, {
		name:      "a value actually changed",
		stored:    map[string]interface{}{"id": "a", "value": "old"},
		refreshed: map[string]interface{}{"id": "a", "value": "new"},
		want:      true,
	}, {
		name:      "an attribute gained a value",
		stored:    map[string]interface{}{"id": "a", "value": "v"},
		refreshed: map[string]interface{}{"id": "a", "value": "v", "label": "now-set"},
		want:      true,
	}, {
		name:      "an attribute lost its value",
		stored:    map[string]interface{}{"id": "a", "label": "was-set"},
		refreshed: map[string]interface{}{"id": "a", "label": nil},
		want:      true,
	}, {
		name:      "the provider stopped reporting an attribute that had a value",
		stored:    map[string]interface{}{"id": "a", "label": "was-set"},
		refreshed: map[string]interface{}{"id": "a"},
		want:      true,
	}, {
		name:      "the provider stopped reporting a nil attribute",
		stored:    map[string]interface{}{"id": "a", "label": nil},
		refreshed: map[string]interface{}{"id": "a"},
		want:      false,
	}, {
		name:      "nested values compare by value",
		stored:    map[string]interface{}{"tags": map[string]interface{}{"env": "prod"}},
		refreshed: map[string]interface{}{"tags": map[string]interface{}{"env": "prod"}},
		want:      false,
	}, {
		name:      "a nested value changed",
		stored:    map[string]interface{}{"tags": map[string]interface{}{"env": "prod"}},
		refreshed: map[string]interface{}{"tags": map[string]interface{}{"env": "dev"}},
		want:      true,
	}, {
		name:      "both empty",
		stored:    map[string]interface{}{},
		refreshed: map[string]interface{}{},
		want:      false,
	}}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := state.Drifted(c.stored, c.refreshed); got != c.want {
				t.Errorf("Drifted(%v, %v) = %v, want %v", c.stored, c.refreshed, got, c.want)
			}
		})
	}
}
