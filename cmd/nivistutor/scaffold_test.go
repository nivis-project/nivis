// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// each embedded starter writes the expected files (flake.nix + config + README),
// with the nivis ref substituted into the flake.
func TestScaffoldWritesStarterFiles(t *testing.T) {
	ts, err := listTutorials()
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) < 2 {
		t.Fatalf("expected at least two tutorials (getting-started + a features one), got %d: %+v", len(ts), ts)
	}
	for _, tut := range ts {
		t.Run(tut.Name, func(t *testing.T) {
			dir := t.TempDir()
			written, err := scaffold(tut.Name, scaffoldOptions{Dir: dir, NivisRef: "github:nivis-project/nivis/v9.9.9"})
			if err != nil {
				t.Fatalf("scaffold: %v", err)
			}
			// flake.nix, config.nix and README.md must all be written.
			want := map[string]bool{"flake.nix": false, "config.nix": false, "README.md": false}
			for _, w := range written {
				if _, ok := want[w]; ok {
					want[w] = true
				}
				if _, err := os.Stat(filepath.Join(dir, w)); err != nil {
					t.Errorf("written file missing on disk: %s (%v)", w, err)
				}
			}
			for f, seen := range want {
				if !seen {
					t.Errorf("starter %q did not write %s", tut.Name, f)
				}
			}
			// The nivis ref placeholder is replaced in flake.nix.
			fl, err := os.ReadFile(filepath.Join(dir, "flake.nix"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(fl), nivisRefPlaceholder) {
				t.Errorf("flake.nix still contains the %s placeholder", nivisRefPlaceholder)
			}
			if !strings.Contains(string(fl), "github:nivis-project/nivis/v9.9.9") {
				t.Errorf("flake.nix did not pin the supplied nivis ref:\n%s", fl)
			}
		})
	}
}

// scaffolding refuses to overwrite an existing file without --force, and leaves
// the existing file untouched (atomic: nothing else is written either).
func TestScaffoldNoClobber(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "config.nix")
	if err := os.WriteFile(existing, []byte("USER EDIT"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scaffold("getting-started", scaffoldOptions{Dir: dir}); err == nil {
		t.Fatal("expected an error scaffolding over an existing file without --force")
	}
	// The user's file is unchanged.
	got, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "USER EDIT" {
		t.Errorf("existing file was modified despite no --force: %q", got)
	}
	// Nothing else was written (no flake.nix appeared).
	if _, err := os.Stat(filepath.Join(dir, "flake.nix")); err == nil {
		t.Error("flake.nix was written despite the no-clobber abort (not atomic)")
	}

	// With --force it overwrites.
	if _, err := scaffold("getting-started", scaffoldOptions{Dir: dir, Force: true}); err != nil {
		t.Fatalf("scaffold --force: %v", err)
	}
	got, _ = os.ReadFile(existing)
	if string(got) == "USER EDIT" {
		t.Error("--force did not overwrite the existing file")
	}
}

// an unknown tutorial name is rejected (no files written).
func TestScaffoldUnknownTutorial(t *testing.T) {
	dir := t.TempDir()
	if _, err := scaffold("does-not-exist", scaffoldOptions{Dir: dir}); err == nil {
		t.Fatal("expected an error for an unknown tutorial")
	}
}

// every embedded tutorial has a flake.nix and a config.nix (guard against a
// tutorial added without its starter scaffolding files).
func TestEmbeddedStartersComplete(t *testing.T) {
	ts, err := listTutorials()
	if err != nil {
		t.Fatal(err)
	}
	for _, tut := range ts {
		for _, f := range []string{"flake.nix", "config.nix", "README.md"} {
			if _, err := embeddedTutorials.ReadFile(tutorialsRoot + "/" + tut.Name + "/" + f); err != nil {
				t.Errorf("tutorial %q is missing %s: %v", tut.Name, f, err)
			}
		}
	}
}

// the menu offers at least the getting-started and a features tutorial.
func TestMenuHasGettingStartedAndFeatures(t *testing.T) {
	ts, err := listTutorials()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	hasGettingStarted, hasFeatures := false, false
	for _, tut := range ts {
		names = append(names, tut.Name)
		if tut.Name == "getting-started" {
			hasGettingStarted = true
		}
		if strings.HasPrefix(tut.Name, "features-") {
			hasFeatures = true
		}
	}
	sort.Strings(names)
	if !hasGettingStarted {
		t.Errorf("no getting-started tutorial in %v", names)
	}
	if !hasFeatures {
		t.Errorf("no features-<version> tutorial in %v", names)
	}
}
