// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase

// The __build leaf path: reading the leaf, choosing what to realise, skipping
// what is already built, verifying the result, and reporting progress.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// stubRealiser records the builds it was asked for and, like a real build,
// PRODUCES the output path — the driver verifies the path afterwards, so a stub
// that only returned nil would not model a build at all.
type stubRealiser struct {
	realised []Build
	// failOn fails the realise of a build whose Path matches.
	failOn string
	// dontCreate makes the realise "succeed" without producing the path, to
	// exercise the post-realise verification.
	dontCreate bool
}

func (s *stubRealiser) Realise(_ context.Context, b Build) error {
	s.realised = append(s.realised, b)
	if s.failOn != "" && b.Path == s.failOn {
		return errBoom
	}
	if s.dontCreate {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(b.Path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(b.Path, []byte("built"), 0o644)
}

var errBoom = &stubError{"boom"}

type stubError struct{ s string }

func (e *stubError) Error() string { return e.s }

// leaf builds a __build leaf carrying an output path and (optionally) a drv.
func leaf(path, drv string) map[string]interface{} {
	b := map[string]interface{}{"path": path}
	if drv != "" {
		b["drv"] = drv
	}
	return map[string]interface{}{"__build": b}
}

// __build leaves are replaced by their path, at any depth, and each one's
// DERIVATION is what gets realised when the leaf carries one.
func TestRealiseBuildsSubstitutes(t *testing.T) {
	dir := t.TempDir()
	imgPath := filepath.Join(dir, "img", "x.vhd")
	pkgPath := filepath.Join(dir, "pkg")

	sr := &stubRealiser{}
	d := &Driver{Realiser: sr}
	cfg := map[string]interface{}{
		"source": leaf(imgPath, "/nix/store/ddd-img.drv"),
		"nested": []interface{}{
			map[string]interface{}{"inner": leaf(pkgPath, "/nix/store/eee-pkg.drv")},
		},
		"plain": "untouched",
	}
	if err := d.realiseBuilds(context.Background(), "r", cfg); err != nil {
		t.Fatal(err)
	}

	if cfg["source"] != imgPath {
		t.Errorf("source not substituted: %#v", cfg["source"])
	}
	inner := cfg["nested"].([]interface{})[0].(map[string]interface{})["inner"]
	if inner != pkgPath {
		t.Errorf("nested leaf not substituted: %#v", inner)
	}
	if cfg["plain"] != "untouched" {
		t.Errorf("plain value changed: %#v", cfg["plain"])
	}

	want := []Build{
		{Path: imgPath, Drv: "/nix/store/ddd-img.drv"},
		{Path: pkgPath, Drv: "/nix/store/eee-pkg.drv"},
	}
	got := append([]Build{}, sr.realised...)
	sortBuilds(got)
	sortBuilds(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("realised = %v, want %v", got, want)
	}
}

// The derivation is what gets realised when present; the output root only when
// the leaf predates that field (and then it is not buildable).
func TestRealiseTargetPrefersTheDerivation(t *testing.T) {
	cases := []struct {
		name          string
		build         Build
		wantTarget    string
		wantBuildable bool
	}{
		{
			name:          "with a derivation",
			build:         Build{Path: "/nix/store/aaa-img/x.vhd", Drv: "/nix/store/bbb-img.drv"},
			wantTarget:    "/nix/store/bbb-img.drv",
			wantBuildable: true,
		},
		{
			name:          "legacy leaf falls back to the output root",
			build:         Build{Path: "/nix/store/aaa-img/x.vhd"},
			wantTarget:    "/nix/store/aaa-img",
			wantBuildable: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target, buildable := realiseTarget(tc.build)
			if target != tc.wantTarget || buildable != tc.wantBuildable {
				t.Errorf("realiseTarget = (%q, %v), want (%q, %v)", target, buildable, tc.wantTarget, tc.wantBuildable)
			}
		})
	}
}

// A path that is already there is not realised again: a built stack costs no
// subprocesses.
func TestRealiseBuildsSkipsAnAlreadyBuiltPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "already")
	if err := os.WriteFile(path, []byte("built"), 0o644); err != nil {
		t.Fatal(err)
	}

	sr := &stubRealiser{}
	d := &Driver{Realiser: sr}
	cfg := map[string]interface{}{"source": leaf(path, "/nix/store/ddd.drv")}
	if err := d.realiseBuilds(context.Background(), "r", cfg); err != nil {
		t.Fatal(err)
	}
	if len(sr.realised) != 0 {
		t.Errorf("an existing path should not be realised; realised=%v", sr.realised)
	}
	if cfg["source"] != path {
		t.Errorf("path should still be substituted: %#v", cfg["source"])
	}
}

// --no-build substitutes the path but does NOT realise.
func TestRealiseBuildsNoBuildSkips(t *testing.T) {
	sr := &stubRealiser{}
	var progress bytes.Buffer
	d := &Driver{Realiser: sr, NoBuild: true, Progress: &progress}
	cfg := map[string]interface{}{"source": leaf("/nix/store/aaa-img/x.vhd", "/nix/store/bbb.drv")}
	if err := d.realiseBuilds(context.Background(), "r", cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["source"] != "/nix/store/aaa-img/x.vhd" {
		t.Errorf("source should still be substituted with --no-build; got %#v", cfg["source"])
	}
	if len(sr.realised) != 0 {
		t.Errorf("--no-build must not realise; realised=%v", sr.realised)
	}
	if progress.Len() != 0 {
		t.Errorf("--no-build must report no build; got %q", progress.String())
	}
}

// A realise failure surfaces as an error naming the path.
func TestRealiseBuildsFailureSurfaces(t *testing.T) {
	sr := &stubRealiser{failOn: "/nix/store/aaa-img/x.vhd"}
	d := &Driver{Realiser: sr}
	cfg := map[string]interface{}{"source": leaf("/nix/store/aaa-img/x.vhd", "/nix/store/bbb.drv")}
	err := d.realiseBuilds(context.Background(), "r", cfg)
	if err == nil {
		t.Fatal("expected a realise error")
	}
	if !strings.Contains(err.Error(), "/nix/store/aaa-img/x.vhd") {
		t.Errorf("error should name the path; got %v", err)
	}
}

// A build that "succeeds" without producing the recorded path is an error naming
// both paths — not a missing file handed to the provider.
func TestRealiseBuildsVerifiesThePathAfterwards(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "never-appears")

	sr := &stubRealiser{dontCreate: true}
	d := &Driver{Realiser: sr}
	cfg := map[string]interface{}{"source": leaf(path, "/nix/store/bbb-img.drv")}
	err := d.realiseBuilds(context.Background(), "r", cfg)
	if err == nil {
		t.Fatal("expected an error when the build did not produce the path")
	}
	for _, want := range []string{path, "/nix/store/bbb-img.drv", "still missing"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q; got %v", want, err)
		}
	}
}

// A build reports that it started and that it finished: a realise produces no
// output of its own, and silence is indistinguishable from a hang.
func TestRealiseBuildsReportsProgress(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "abcdef0123456789-nixos-image", "disk.vhd")

	var progress bytes.Buffer
	d := &Driver{Realiser: &stubRealiser{}, Progress: &progress}
	cfg := map[string]interface{}{"source": leaf(path, "/nix/store/bbb-img.drv")}
	if err := d.realiseBuilds(context.Background(), "owner.res.x", cfg); err != nil {
		t.Fatal(err)
	}
	// The label is the store-path name for a real /nix/store path (see
	// TestStoreName); for this temp path it is the file's own name.
	out := progress.String()
	for _, want := range []string{"Building", "disk.vhd", "owner.res.x", "Built"} {
		if !strings.Contains(out, want) {
			t.Errorf("progress output lacks %q:\n%s", want, out)
		}
	}
}

// A nil Progress writer discards rather than panicking.
func TestRealiseBuildsWithoutProgressWriter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x")
	d := &Driver{Realiser: &stubRealiser{}}
	cfg := map[string]interface{}{"source": leaf(path, "/nix/store/bbb.drv")}
	if err := d.realiseBuilds(context.Background(), "r", cfg); err != nil {
		t.Fatal(err)
	}
}

// Reading a leaf: both shapes, and non-leaves left alone.
func TestBuildLeafParsing(t *testing.T) {
	got, ok := buildLeaf(leaf("/nix/store/aaa-img/x.vhd", "/nix/store/bbb.drv"))
	if !ok || got.Path != "/nix/store/aaa-img/x.vhd" || got.Drv != "/nix/store/bbb.drv" {
		t.Errorf("modern leaf = (%+v, %v)", got, ok)
	}
	got, ok = buildLeaf(leaf("/nix/store/aaa-img", ""))
	if !ok || got.Path != "/nix/store/aaa-img" || got.Drv != "" {
		t.Errorf("legacy leaf = (%+v, %v)", got, ok)
	}
	for _, notALeaf := range []map[string]interface{}{
		{"other": "value"},
		{"__build": "not an object"},
		{"__build": map[string]interface{}{}},                    // no path
		{"__build": map[string]interface{}{"path": ""}},          // empty path
		{"__build": map[string]interface{}{"path": 42}},          // wrong type
		{"__build": map[string]interface{}{"drv": "/nix/x.drv"}}, // drv without path
	} {
		if _, ok := buildLeaf(notALeaf); ok {
			t.Errorf("%v should not read as a build leaf", notALeaf)
		}
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

func TestStoreName(t *testing.T) {
	cases := map[string]string{
		"/nix/store/abc123-nixos-image/disk.vhd": "nixos-image",
		"/nix/store/abc123-nixos-image":          "nixos-image",
		"/nix/store/nohyphen":                    "nohyphen",
		"/etc/passwd":                            "passwd",
	}
	for in, want := range cases {
		if got := storeName(in); got != want {
			t.Errorf("storeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func sortBuilds(b []Build) {
	for i := 1; i < len(b); i++ {
		for j := i; j > 0 && b[j-1].Path > b[j].Path; j-- {
			b[j-1], b[j] = b[j], b[j-1]
		}
	}
}
