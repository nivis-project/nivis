// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// build the tn binary once for the error-UX tests.
func buildCLI(t *testing.T) string {
	t.Helper()
	root := repoRoot(t)
	bin := filepath.Join(t.TempDir(), "tn")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/tn")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot build tn: %v\n%s", err, out)
	}
	return bin
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		p := filepath.Dir(dir)
		if p == dir {
			t.Fatal("go.mod not found")
		}
		dir = p
	}
}

// A runtime failure (unknown flake attr) must print a clean `error:` line and
// NOT dump the command usage block.
func TestRuntimeErrorIsClean(t *testing.T) {
	if _, err := exec.LookPath("nix"); err != nil {
		t.Skip("nix not on PATH")
	}
	bin := buildCLI(t)
	cmd := exec.Command(bin, "plan", "--attr", "terraeNivis.doesNotExist")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected a non-zero exit for an unknown attribute")
	}
	s := string(out)
	if !strings.Contains(s, "error:") {
		t.Errorf("output should contain an error: line; got:\n%s", s)
	}
	// no usage dump on a runtime error
	if strings.Contains(s, "Usage:") || strings.Contains(s, "Flags:") {
		t.Errorf("runtime error must NOT print usage; got:\n%s", s)
	}
	// dirty-tree warning must be filtered out
	if strings.Contains(s, "Git tree") {
		t.Errorf("nix dirty-tree warning should be filtered; got:\n%s", s)
	}
}

// Flag misuse (unknown flag) is a parse error and SHOULD show usage.
func TestFlagMisuseShowsUsage(t *testing.T) {
	bin := buildCLI(t)
	cmd := exec.Command(bin, "plan", "--no-such-flag")
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected a non-zero exit for an unknown flag")
	}
	if !strings.Contains(string(out), "Usage:") {
		t.Errorf("flag misuse should show usage; got:\n%s", out)
	}
}
