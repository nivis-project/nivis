// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlanApplyOutputFidelity is the regression guard for the two plan/apply
// output-fidelity bugs surfaced by the features-0.4 tutorial (beans-z57y, oh90).
// It drives the REAL nivis binary against the bundled `nivis.tutorial` attr (a
// datasource feeding a resource, a round trip into a second), and asserts on the
// user-facing CLI output:
//   - a re-plan reports the datasource-dependent resource as no-op (=), not ~ update;
//   - a re-apply reports resources as no-op (=), not + create.
//
// NO_COLOR makes the output stable (no ANSI codes) for assertions.
func TestPlanApplyOutputFidelity(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root) // provider-alpha/beta on $PATH (bare-name source)

	// Build the real nivis CLI.
	bin := filepath.Join(t.TempDir(), "nivis")
	build := exec.Command("go", "build", "-o", bin, "./cmd/nivis")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("cannot build nivis: %v\n%s", err, out)
	}

	statePath := filepath.Join(t.TempDir(), "state.json")
	run := func(args ...string) string {
		t.Helper()
		full := append([]string{}, args...)
		full = append(full, "--attr", "nivis.tutorial", "--state", statePath, "--var", "env=prod")
		cmd := exec.Command(bin, full...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "NO_COLOR=1") // stable, ANSI-free output
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("nivis %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}

	// Apply #1: the resources are created.
	apply1 := run("apply")
	if !strings.Contains(apply1, "+ alpha.alpha_token.app") {
		t.Errorf("apply #1 should create alpha.alpha_token.app (+):\n%s", apply1)
	}

	// Re-PLAN: the datasource-dependent resource must report no-op (=), NOT ~ update
	// (the oh90 bug), and the plan must report no changes.
	plan2 := run("plan")
	if strings.Contains(plan2, "~ alpha.alpha_token.app") {
		t.Errorf("re-plan wrongly reports a ~ update for the datasource-dependent resource (oh90):\n%s", plan2)
	}
	if !strings.Contains(plan2, "= alpha.alpha_token.app") {
		t.Errorf("re-plan should report alpha.alpha_token.app as no-op (=):\n%s", plan2)
	}
	if !strings.Contains(plan2, "No changes.") {
		t.Errorf("re-plan of an unchanged stack should report No changes:\n%s", plan2)
	}

	// Re-APPLY: resources report no-op (=), NOT + create (the z57y bug).
	apply2 := run("apply")
	if strings.Contains(apply2, "+ alpha.alpha_token.app") || strings.Contains(apply2, "+ beta.beta_record.app") {
		t.Errorf("re-apply wrongly reports a + create for an unchanged resource (z57y):\n%s", apply2)
	}
	if !strings.Contains(apply2, "= alpha.alpha_token.app") {
		t.Errorf("re-apply should report alpha.alpha_token.app as no-op (=):\n%s", apply2)
	}

	// Sanity: the output is ANSI-free under NO_COLOR (stable for assertions).
	if strings.Contains(apply2, "\x1b[") {
		t.Errorf("NO_COLOR output contains ANSI escape codes:\n%q", apply2)
	}
}
