// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"sort"
	"testing"
)

// The tutorial menu order is controlled (beans-x3v1): getting-started first, then
// features-<version> newest-version-first (numeric, so 0.10 > 0.5 > 0.4), then any
// others alphabetically. Uses synthetic names so the version compare is exercised
// beyond the two tutorials that ship today.
func TestTutorialOrder(t *testing.T) {
	names := []string{
		"features-0.4",
		"zeta-extra",
		"features-0.10",
		"getting-started",
		"features-0.5",
		"aardvark-extra",
	}
	sort.Slice(names, func(i, j int) bool { return lessTutorial(names[i], names[j]) })

	want := []string{
		"getting-started", // entry point, always first
		"features-0.10",   // features, newest version first (numeric: 0.10 > 0.5 > 0.4)
		"features-0.5",
		"features-0.4",
		"aardvark-extra", // others, alphabetical
		"zeta-extra",
	}
	if len(names) != len(want) {
		t.Fatalf("len mismatch: %v", names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("order[%d] = %q, want %q\n got: %v\nwant: %v", i, names[i], want[i], names, want)
		}
	}
}

// The two tutorials that ship today list getting-started first.
func TestEmbeddedMenuGettingStartedFirst(t *testing.T) {
	ts, err := listTutorials()
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) == 0 {
		t.Fatal("no tutorials embedded")
	}
	if ts[0].Name != "getting-started" {
		t.Errorf("first tutorial = %q, want getting-started", ts[0].Name)
	}
}

// Numeric version compare: 0.10 is greater than 0.9 (not lexically smaller).
func TestCompareVersionNumeric(t *testing.T) {
	if compareVersion(parseVersion("0.10"), parseVersion("0.9")) <= 0 {
		t.Error("0.10 should compare greater than 0.9 (numeric, not lexical)")
	}
	if compareVersion(parseVersion("1.0"), parseVersion("1.0.0")) != 0 {
		t.Error("1.0 and 1.0.0 should compare equal")
	}
}
