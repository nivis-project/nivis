// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package e2e_test

// Provider notes end to end, through the REAL nivis binary against the
// log-emitting fake (cmd/provider-zeta): provider stderr -> the plugin
// transport's parse -> the renderer -> the terminal. Nothing is simulated except
// the provider, and the entry it emits is the verbatim real-world one from bean
// nixform2-ceoh.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// providerLogRun builds the CLI and returns a runner over the `nivis.providerLog`
// attr, which declares `--var count` resources over provider-zeta.
func providerLogRun(t *testing.T, root string) func(t *testing.T, args ...string) (string, error) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "nivis")
	build := exec.Command("go", "build", "-o", bin, "./cmd/nivis")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("cannot build nivis: %v\n%s", err, out)
	}
	statePath := filepath.Join(t.TempDir(), "nivis.state.json")

	return func(t *testing.T, args ...string) (string, error) {
		t.Helper()
		full := append([]string{}, args...)
		full = append(full, "--attr", "nivis.providerLog", "--state", statePath)
		cmd := exec.Command(bin, full...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "NO_COLOR=1")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
}

// The bean's three problems, asserted at the user's terminal: readable, not
// alarming, not repeated.
//
// The run is an `apply`: a plan of a stack that is not yet in state reports
// creates without contacting the provider at all (PlanReport only calls the
// provider for resources it finds in state), so a fresh `plan` spawns nothing
// and there is no provider output to render. Apply is the first run that
// actually reaches PlanResourceChange; a re-plan of the applied stack does too,
// which the collapse test below uses.
func TestProviderNoteIsReadableAndNotAFailure(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root) // provider-zeta on $PATH
	run := providerLogRun(t, root)

	out, err := run(t, "apply", "--var", "count=1")
	if err != nil {
		t.Fatalf("apply should SUCCEED while emitting a provider note: %v\n%s", err, out)
	}

	// Readable: one line, naming the subject and the message.
	for _, want := range []string{
		"provider note",
		"zeta_note.description",
		"unable to require attribute replacement",
		"detail: ForceNew: No changes for description",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}

	// Not alarming: the note must not read as an error, and the run succeeded.
	if strings.Contains(out, "error=") || strings.Contains(out, "error:") {
		t.Errorf("a warn-level note must not be rendered as an error:\n%s", out)
	}
	if strings.Contains(out, "provider error") {
		t.Errorf("a warn-level note was marked as a provider error:\n%s", out)
	}

	// The provider's internal telemetry is gone, including the raw hclog line.
	for _, unwanted := range []string{
		"tf_req_id", "dd296e15", "@caller", "force_new.go", "tf_mux_provider",
		"tf_provider_addr", "sdk.helper_schema", "2026-08-31T23:58:27", "[WARN]",
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("output still carries the provider's telemetry %q:\n%s", unwanted, out)
		}
	}

	// The message appears exactly once for a single resource.
	if n := strings.Count(out, "unable to require attribute replacement"); n != 1 {
		t.Errorf("the note appears %d times for one resource, want 1:\n%s", n, out)
	}
}

// The collapse, end to end: several resources, one printed note, a reported
// count for the rest.
func TestProviderNotesCollapseAcrossResources(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	run := providerLogRun(t, root)

	// Apply the stack first, then re-plan it: the re-plan calls the provider for
	// every resource now in state, so the same note arrives four times.
	if out, err := run(t, "apply", "--var", "count=4"); err != nil {
		t.Fatalf("apply: %v\n%s", err, out)
	}
	out, err := run(t, "plan", "--var", "count=4")
	if err != nil {
		t.Fatalf("re-plan: %v\n%s", err, out)
	}
	// Once inline plus once in the end-of-run summary.
	if n := strings.Count(out, "unable to require attribute replacement"); n != 2 {
		t.Errorf("the note appears %d times, want 2 (one note + one summary):\n%s", n, out)
	}
	if !strings.Contains(out, "further occurrence(s) not shown") {
		t.Errorf("the run should report the collapsed repeats:\n%s", out)
	}
}

// The level knob: silence at error, unabridged at trace.
func TestProviderLogLevelEndToEnd(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	run := providerLogRun(t, root)

	quiet, err := run(t, "apply", "--var", "count=1", "--provider-log-level=error")
	if err != nil {
		t.Fatalf("apply at error level: %v\n%s", err, quiet)
	}
	if strings.Contains(quiet, "unable to require attribute replacement") {
		t.Errorf("--provider-log-level=error should surface no warn note:\n%s", quiet)
	}
	if !strings.Contains(quiet, "zeta.zeta_note.n0") {
		t.Errorf("the change list should still be there:\n%s", quiet)
	}

	verbatim, err := run(t, "plan", "--var", "count=1", "--provider-log-level=trace")
	if err != nil {
		t.Fatalf("re-plan at trace level: %v\n%s", err, verbatim)
	}
	for _, want := range []string{"tf_req_id", "dd296e15", "tf_rpc"} {
		if !strings.Contains(verbatim, want) {
			t.Errorf("trace should restore %q for debugging:\n%s", want, verbatim)
		}
	}
}

// An unknown level is refused before anything is spawned.
func TestProviderLogLevelUnknownIsRefusedByTheCLI(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)
	run := providerLogRun(t, root)

	out, err := run(t, "plan", "--provider-log-level=verbose")
	if err == nil {
		t.Fatalf("an unknown level should fail the command:\n%s", out)
	}
	for _, want := range []string{"verbose", "warn", "trace"} {
		if !strings.Contains(out, want) {
			t.Errorf("the error should name %q:\n%s", want, out)
		}
	}
}

// Notes go to stderr, the change list to stdout: redirecting the change list
// keeps it unmixed and machine-readable.
func TestProviderNotesGoToStderrNotStdout(t *testing.T) {
	requireNix(t)
	root := repoRoot(t)
	buildBinaries(t, root)

	bin := filepath.Join(t.TempDir(), "nivis")
	build := exec.Command("go", "build", "-o", bin, "./cmd/nivis")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("cannot build nivis: %v\n%s", err, out)
	}

	statePath := filepath.Join(t.TempDir(), "s.json")
	// Apply first so the following plan actually reaches the provider.
	seed := exec.Command(bin, "apply", "--attr", "nivis.providerLog", "--var", "count=1", "--state", statePath)
	seed.Dir = root
	seed.Env = append(os.Environ(), "NO_COLOR=1")
	if out, err := seed.CombinedOutput(); err != nil {
		t.Fatalf("seed apply: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "plan", "--attr", "nivis.providerLog", "--var", "count=1",
		"--state", statePath)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "NO_COLOR=1")
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("plan: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	if strings.Contains(stdout.String(), "provider note") {
		t.Errorf("provider notes must not pollute the change list on stdout:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "provider note") {
		t.Errorf("provider notes should be on stderr:\n%s", stderr.String())
	}
	if !strings.Contains(stdout.String(), "zeta.zeta_note.n0") {
		t.Errorf("the change list should be on stdout:\n%s", stdout.String())
	}
}
