// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package e2e_test

// The three output channels, asserted end to end against the REAL nivis binary:
//
//   - stdout is THE RESULT and is byte-identical whether or not a terminal is
//     attached, so a pipeline sees what a file does.
//   - stderr is THE NARRATIVE and reports work as it completes.
//   - the live region never reaches redirected output.

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// nivisBin builds the real CLI once for a test.
func nivisBin(t *testing.T, root string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "nivis")
	build := exec.Command("go", "build", "-o", bin, "./cmd/nivis")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("cannot build nivis: %v\n%s", err, out)
	}
	return bin
}

// runSplit runs nivis with stdout and stderr captured SEPARATELY, which is the
// whole point: the channel split is invisible to CombinedOutput.
func runSplit(t *testing.T, bin, root, statePath string, env []string, args ...string) (string, string) {
	t.Helper()
	full := append([]string{}, args...)
	full = append(full, "--attr", "nivis.tutorial", "--state", statePath, "--var", "env=prod")
	cmd := exec.Command(bin, full...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("nivis %s: %v\nstdout:\n%s\nstderr:\n%s",
			strings.Join(args, " "), err, stdout.String(), stderr.String())
	}
	return stdout.String(), stderr.String()
}

// The result channel carries the change list and the summary; the narrative
// channel carries progress. Neither leaks into the other.
func TestResultAndNarrativeAreSeparateChannels(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	bin := nivisBin(t, root)
	statePath := filepath.Join(t.TempDir(), "state.json")

	stdout, stderr := runSplit(t, bin, root, statePath, []string{"NO_COLOR=1"}, "apply")

	// The result is on stdout.
	if !strings.Contains(stdout, "Applied") {
		t.Errorf("the summary belongs on stdout:\nstdout:\n%s", stdout)
	}
	if !strings.Contains(stdout, "alpha.alpha_token.app") {
		t.Errorf("the change list belongs on stdout:\nstdout:\n%s", stdout)
	}

	// The narrative is on stderr, and reports work as it completes.
	if !strings.Contains(stderr, "alpha.alpha_token.app") {
		t.Errorf("the narrative should report each node as it completes:\nstderr:\n%s", stderr)
	}
	if !strings.Contains(stderr, "Phase 1") {
		t.Errorf("the narrative should report phase boundaries:\nstderr:\n%s", stderr)
	}
}

// stdout is byte-identical with and without a terminal. This is the contract
// that lets `nivis apply > result.txt` keep working, and the reason the result
// is rendered by code that does not know a renderer exists.
func TestStdoutIsIdenticalRegardlessOfVerbosity(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	bin := nivisBin(t, root)

	// Apply once per state file so each run plans the same way.
	first := filepath.Join(t.TempDir(), "state.json")
	second := filepath.Join(t.TempDir(), "state.json")

	quietOut, quietErr := runSplit(t, bin, root, first,
		[]string{"NO_COLOR=1", "NIVIS_LOG=quiet"}, "apply")
	verboseOut, verboseErr := runSplit(t, bin, root, second,
		[]string{"NO_COLOR=1", "NIVIS_LOG=verbose"}, "apply")

	if quietOut != verboseOut {
		t.Errorf("stdout must not vary with verbosity — it is the machine-readable result:\n"+
			"quiet:\n%s\nverbose:\n%s", quietOut, verboseOut)
	}
	// And the narrative genuinely did differ, or the comparison above proves
	// nothing.
	if len(verboseErr) <= len(quietErr) {
		t.Errorf("verbose should narrate more than quiet; quiet=%d bytes verbose=%d bytes",
			len(quietErr), len(verboseErr))
	}
	if strings.TrimSpace(quietErr) != "" {
		t.Errorf("quiet should produce no narrative:\n%s", quietErr)
	}
}

// A redirected run is clean: no ANSI escapes and no partially-rewritten status
// lines, because neither stream is a terminal and the live region is never used.
func TestRedirectedRunHasNoTerminalControlSequences(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	bin := nivisBin(t, root)
	statePath := filepath.Join(t.TempDir(), "state.json")

	// Deliberately WITHOUT NO_COLOR: the point is that a non-terminal is
	// detected on its own, not that the user asked for plain output.
	stdout, stderr := runSplit(t, bin, root, statePath, nil, "apply")

	for name, s := range map[string]string{"stdout": stdout, "stderr": stderr} {
		if strings.Contains(s, "\x1b") {
			t.Errorf("%s contains ANSI escapes although it is not a terminal:\n%q", name, s)
		}
		if strings.Contains(s, "\r") {
			t.Errorf("%s contains a carriage return, which means a rewritten status line "+
				"reached redirected output:\n%q", name, s)
		}
	}
}

// The state-lock lines report what HAPPENED, so they belong to the narrative,
// not to the machine-readable result where they used to be printed.
func TestStateLockIsNarrativeNotResult(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	bin := nivisBin(t, root)
	statePath := filepath.Join(t.TempDir(), "state.json")

	stdout, _ := runSplit(t, bin, root, statePath, []string{"NO_COLOR=1"}, "apply")
	if strings.Contains(stdout, "state lock") {
		t.Errorf("the state-lock report belongs on the narrative channel, not in the result:\n%s", stdout)
	}
}
