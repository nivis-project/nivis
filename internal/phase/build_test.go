// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

// stubRealiser records the store roots it was asked to realise and can fail.
type stubRealiser struct {
	realised []string
	failOn   string
}

func (s *stubRealiser) Realise(_ context.Context, storePath string) error {
	s.realised = append(s.realised, storePath)
	if s.failOn != "" && storePath == s.failOn {
		return fmt.Errorf("boom")
	}
	return nil
}

func buildLeaf(path string) map[string]interface{} {
	return map[string]interface{}{"__build": map[string]interface{}{"path": path}}
}

// realiseBuilds realises each __build leaf's store ROOT and substitutes the full
// path, recursing through nested maps and lists.
func TestRealiseBuildsSubstitutes(t *testing.T) {
	sr := &stubRealiser{}
	d := &Driver{Realiser: sr}
	cfg := map[string]interface{}{
		"source": buildLeaf("/nix/store/aaa-img/x.vhd"),
		"nested": []interface{}{
			map[string]interface{}{"inner": buildLeaf("/nix/store/bbb-pkg")},
		},
		"plain": "untouched",
	}
	if err := d.realiseBuilds(context.Background(), "r", cfg); err != nil {
		t.Fatal(err)
	}
	// leaves replaced by their path strings
	if cfg["source"] != "/nix/store/aaa-img/x.vhd" {
		t.Errorf("source not substituted: %#v", cfg["source"])
	}
	inner := cfg["nested"].([]interface{})[0].(map[string]interface{})["inner"]
	if inner != "/nix/store/bbb-pkg" {
		t.Errorf("nested leaf not substituted: %#v", inner)
	}
	if cfg["plain"] != "untouched" {
		t.Errorf("plain value changed: %#v", cfg["plain"])
	}
	// realised the store ROOTS (not the inner file path)
	want := []string{"/nix/store/aaa-img", "/nix/store/bbb-pkg"}
	got := append([]string{}, sr.realised...)
	sortStrings(got)
	sortStrings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("realised roots = %v, want %v", got, want)
	}
}

// --no-build (NoBuild) substitutes the path but does NOT realise.
func TestRealiseBuildsNoBuildSkips(t *testing.T) {
	sr := &stubRealiser{}
	d := &Driver{Realiser: sr, NoBuild: true}
	cfg := map[string]interface{}{"source": buildLeaf("/nix/store/aaa-img/x.vhd")}
	if err := d.realiseBuilds(context.Background(), "r", cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["source"] != "/nix/store/aaa-img/x.vhd" {
		t.Errorf("source should still be substituted with --no-build; got %#v", cfg["source"])
	}
	if len(sr.realised) != 0 {
		t.Errorf("--no-build must not realise; realised=%v", sr.realised)
	}
}

// a realise failure surfaces as an error naming the path.
func TestRealiseBuildsFailureSurfaces(t *testing.T) {
	sr := &stubRealiser{failOn: "/nix/store/aaa-img"}
	d := &Driver{Realiser: sr}
	cfg := map[string]interface{}{"source": buildLeaf("/nix/store/aaa-img/x.vhd")}
	err := d.realiseBuilds(context.Background(), "r", cfg)
	if err == nil {
		t.Fatal("expected a realise error")
	}
	if !contains(err.Error(), "/nix/store/aaa-img") {
		t.Errorf("error should name the path; got %v", err)
	}
}

func TestStoreRoot(t *testing.T) {
	cases := map[string]string{
		"/nix/store/h-name/sub/file.vhd": "/nix/store/h-name",
		"/nix/store/h-name":              "/nix/store/h-name",
		"/etc/passwd":                    "/etc/passwd", // non-store: unchanged
	}
	for in, want := range cases {
		if got := storeRoot(in); got != want {
			t.Errorf("storeRoot(%q) = %q, want %q", in, got, want)
		}
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
