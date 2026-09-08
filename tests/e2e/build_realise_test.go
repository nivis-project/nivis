// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package e2e_test

// `nivis apply` BUILDS a __build leaf, it does not merely substitute it.
//
// This test exists because the previous proof did not prove: the only test of the
// build path used a stub realiser, and every real-world run had the output path
// already valid (an eval-time `builtins.hashFile`, an earlier `nix build`, or a
// pre-made AMI), so `nix-store --realise <outputPath>` was always a no-op. An
// output path names a result, not a recipe; it cannot be built.
//
// The guard against this test decaying the same way is the probe's NAME: it is
// unique per run, so the derivation is in no store and reachable from no
// substituter, and the test asserts that BEFORE it runs anything. With a fixed
// name the first run would build the probe and every later run would silently
// verify nothing.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// probeToken returns a token unique to this test run, for the probe derivation's
// name. Go's temp dir carries a per-run random component, which is exactly the
// uniqueness this needs.
func probeToken(t *testing.T, suffix string) string {
	t.Helper()
	base := filepath.Base(filepath.Dir(t.TempDir())) // e.g. TestX2895418321
	var b strings.Builder
	for _, r := range base + "-" + suffix {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// buildLeafFor evaluates the probe config and returns the __build leaf's output
// and derivation paths. Reading them from the CONFIG (rather than duplicating the
// derivation in the test) means the test cannot drift from what nivis will see —
// and it proves the leaf carries both paths at all.
func buildLeafFor(t *testing.T, root, token string) (outPath, drvPath string) {
	t.Helper()
	apply := `p: builtins.toJSON (p { outputs = {}; vars = { probe_token = "` + token + `"; }; })`
	cmd := exec.Command("nix", "eval", ".#nivis.buildProbe", "--impure", "--apply", apply, "--raw")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("evaluating nivis.buildProbe: %v", err)
	}

	var ir struct {
		Resources []struct {
			Config struct {
				Label struct {
					Build struct {
						Path string `json:"path"`
						Drv  string `json:"drv"`
					} `json:"__build"`
				} `json:"label"`
			} `json:"config"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(out, &ir); err != nil {
		t.Fatalf("parsing the IR: %v\n%s", err, out)
	}
	if len(ir.Resources) != 1 {
		t.Fatalf("expected one resource, got %d", len(ir.Resources))
	}
	leaf := ir.Resources[0].Config.Label.Build
	if leaf.Path == "" {
		t.Fatalf("the __build leaf carries no output path:\n%s", out)
	}
	if leaf.Drv == "" {
		t.Fatalf("the __build leaf carries no DERIVATION path — an output path alone "+
			"cannot be built, which is the bug this test covers:\n%s", out)
	}
	return leaf.Path, leaf.Drv
}

// storePathValid reports whether a store path is valid (built).
func storePathValid(path string) bool {
	return exec.Command("nix-store", "--check-validity", path).Run() == nil
}

// buildNivis builds the CLI and returns its path.
func buildNivis(t *testing.T, root string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "nivis")
	build := exec.Command("go", "build", "-o", bin, "./cmd/nivis")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("cannot build nivis: %v\n%s", err, out)
	}
	return bin
}

// The headline: a config whose __build leaf points at a never-built, uncached
// derivation applies in ONE run, with no pre-build step.
func TestApplyBuildsANeverBuiltDerivation(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root) // provider-alpha on $PATH
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")

	token := probeToken(t, "headline")
	outPath, drvPath := buildLeafFor(t, root, token)

	// THE ANTI-DECAY GUARD. If this ever fails, the probe is no longer unique per
	// run and the rest of this test proves nothing.
	if storePathValid(outPath) {
		t.Fatalf("the probe %s is ALREADY BUILT, so this test would verify nothing; "+
			"the probe's name must be unique per run", outPath)
	}

	// Negative control: realising the OUTPUT path cannot work. Asserted on the
	// error text so that a future change which silently makes it "work" is caught
	// rather than quietly making this test tautological.
	neg, err := exec.Command("nix-store", "--realise", outPath).CombinedOutput()
	if err == nil {
		t.Fatalf("realising an output path should be impossible, but it succeeded:\n%s", neg)
	}
	if !strings.Contains(string(neg), "no substituter that can build it") &&
		!strings.Contains(string(neg), "don't know how to build") {
		t.Errorf("unexpected failure realising an output path:\n%s", neg)
	}
	if storePathValid(outPath) {
		t.Fatalf("the failed realise built the probe anyway: %s", outPath)
	}

	// The real thing: one apply, no pre-build.
	statePath := filepath.Join(t.TempDir(), "nivis.state.json")
	cmd := exec.Command(buildNivis(t, root), "apply",
		"--attr", "nivis.buildProbe",
		"--state", statePath,
		"--var", "probe_token="+token)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	applyOut, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("apply should build the probe and succeed: %v\n%s", err, applyOut)
	}

	// The build happened, and said so: a realise produces no output of its own,
	// so silence would be indistinguishable from a hang.
	for _, want := range []string{"Building", "nivis-build-probe", "Built"} {
		if !strings.Contains(string(applyOut), want) {
			t.Errorf("apply output should report the build (%q):\n%s", want, applyOut)
		}
	}

	// The output path is now valid, and holds what the derivation produces.
	if !storePathValid(outPath) {
		t.Fatalf("the probe was not built by the apply: %s\n%s", outPath, applyOut)
	}
	content, readErr := os.ReadFile(outPath)
	if readErr != nil {
		t.Fatalf("reading the built probe: %v", readErr)
	}
	if strings.TrimSpace(string(content)) != "built-by-nivis" {
		t.Errorf("built probe content = %q, want \"built-by-nivis\"", content)
	}

	// The concrete path reached the provider: the fake echoes its label into the
	// value it returns, so the applied state carries the built path.
	state, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	if !strings.Contains(string(state), outPath) {
		t.Errorf("the built path did not reach the provider (absent from state):\n%s", state)
	}
	_ = drvPath
}

// --build=false means what it says: nothing is built, and nothing is reported.
func TestApplyWithBuildDisabledDoesNotBuild(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	t.Setenv("TERRAE_NIVIS_FAKE_COUNTER", "")

	token := probeToken(t, "nobuild")
	outPath, _ := buildLeafFor(t, root, token)
	if storePathValid(outPath) {
		t.Fatalf("the probe %s is already built; the probe name must be unique per run", outPath)
	}

	cmd := exec.Command(buildNivis(t, root), "apply",
		"--attr", "nivis.buildProbe",
		"--state", filepath.Join(t.TempDir(), "nivis.state.json"),
		"--var", "probe_token="+token,
		"--build=false")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("apply --build=false: %v\n%s", err, out)
	}

	if storePathValid(outPath) {
		t.Errorf("--build=false built the probe anyway: %s", outPath)
	}
	if strings.Contains(string(out), "Building") {
		t.Errorf("--build=false should report no build:\n%s", out)
	}
}
