// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package ui

import "testing"

func TestBrandColoursAreTheHouseStyle(t *testing.T) {
	for _, tc := range []struct {
		name, got, want string
	}{
		{"Accent", Accent, "\x1b[38;2;45;210;197m"},
		{"Name", Name, "\x1b[38;2;123;211;255m"},
		{"Dim", Dim, "\x1b[38;2;147;184;199m"},
		{"Bright", Bright, "\x1b[1m"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %q, want %q", tc.name, tc.got, tc.want)
		}
	}
}
