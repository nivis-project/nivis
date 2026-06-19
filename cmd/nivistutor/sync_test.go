// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The embedded starters under cmd/nivistutor/tutorials/<name>/ are a build-time
// copy of the canonical starters at nix/example/tutorial-<name>/ (the
// spec-mandated location the repo flake imports). This test guards against drift
// between the two: if a starter is edited in nix/example/ but not re-copied here
// (or vice versa), nivistutor would scaffold stale files. Keep them in sync with:
//
//	cp -r nix/example/tutorial-<name>/* cmd/nivistutor/tutorials/<name>/
func TestEmbeddedStartersMatchCanonical(t *testing.T) {
	root := repoRoot(t)
	ts, err := listTutorials()
	if err != nil {
		t.Fatal(err)
	}
	for _, tut := range ts {
		canon := filepath.Join(root, "nix", "example", "tutorial-"+tut.Name)
		if _, err := os.Stat(canon); err != nil {
			t.Errorf("tutorial %q has no canonical dir at %s: %v", tut.Name, canon, err)
			continue
		}
		for _, f := range []string{"flake.nix", "config.nix", "README.md"} {
			embedded, eerr := embeddedTutorials.ReadFile(tutorialsRoot + "/" + tut.Name + "/" + f)
			if eerr != nil {
				t.Errorf("embedded %s/%s: %v", tut.Name, f, eerr)
				continue
			}
			onDisk, derr := os.ReadFile(filepath.Join(canon, f))
			if derr != nil {
				t.Errorf("canonical %s/%s: %v", tut.Name, f, derr)
				continue
			}
			if !bytes.Equal(embedded, onDisk) {
				t.Errorf("embedded tutorials/%s/%s differs from canonical nix/example/tutorial-%s/%s; re-copy to sync",
					tut.Name, f, tut.Name, f)
			}
		}
	}
}

// repoRoot walks up from the test's working directory to the module root (the
// dir holding go.mod).
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find go.mod above the test working directory")
		}
		dir = parent
	}
}
