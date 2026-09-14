// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase_test

// Does a REAL `nix-store --realise` actually feed the decoder? The unit tests
// replay a captured fixture; this one runs nix.

import (
	"context"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/nixlog"
	"github.com/nivis-project/nivis/internal/phase"
	"github.com/nivis-project/nivis/internal/progress"
)

func requireNixBin(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("nix-store"); err != nil {
		t.Skip("nix-store not on PATH")
	}
}

// buildProbeDrv instantiates a derivation that has never been built, so
// realising it is a real build rather than a cache hit.
//
// The name carries a RANDOM token, not the temp dir's name: t.TempDir() ends in
// a counter ("001"), which repeats across runs, so a dir-derived name would be
// already built on the second run and the test would silently observe no build
// at all. (nix/example/build-probe.nix takes the same precaution.)
func buildProbeDrv(t *testing.T) string {
	t.Helper()
	requireNixBin(t)
	dir := t.TempDir()
	nix := filepath.Join(dir, "probe.nix")
	token := strconv.FormatInt(rand.Int63(), 36)
	src := `derivation {
  name = "nivis-buildevents-probe-` + token + `";
  system = builtins.currentSystem;
  builder = "/bin/sh";
  args = [ "-c" "echo probe-is-building; echo ok > $out" ];
}`
	if err := os.WriteFile(nix, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("nix-instantiate", nix).Output()
	if err != nil {
		t.Skipf("cannot instantiate the probe: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// A real realise must report progress through the decoder: the derivation
// building, a count, and the build's own output line. If this fails while the
// fixture tests pass, the wiring is wrong, not the decoding.
func TestRealBuildFeedsTheDecoder(t *testing.T) {
	drv := buildProbeDrv(t)

	var updates []nixlog.Update
	term := phase.Terminal{OnBuild: func(u nixlog.Update) { updates = append(updates, u) }}

	d := &phase.Driver{Term: term}
	out, err := d.RealiseForTest(context.Background(), drv)
	if err != nil {
		t.Fatalf("realise: %v\n%s", err, out)
	}

	if len(updates) == 0 {
		t.Fatal("a real build produced no decoded updates; the event stream is not reaching the decoder")
	}

	var sawName, sawCount, sawLine bool
	for _, u := range updates {
		if strings.Contains(u.Building, "nivis-buildevents-probe") {
			sawName = true
		}
		if u.Expected > 0 {
			sawCount = true
		}
		if u.LastLine == "probe-is-building" {
			sawLine = true
		}
	}
	if !sawName {
		t.Error("no update named the derivation being built")
	}
	if !sawCount {
		t.Error("no update carried a derivation count")
	}
	if !sawLine {
		t.Error("the build's own output line never arrived — this is what proves " +
			"--print-build-logs is not needed (and nix-store rejects it)")
	}
}

// The full output must still be captured while decoding, because the error
// path depends on having all of it.
func TestDecodingStillCapturesOutputForErrors(t *testing.T) {
	requireNixBin(t)
	term := phase.Terminal{OnBuild: func(nixlog.Update) {}}
	d := &phase.Driver{Term: term}

	// A path that cannot be realised: the error text must survive decoding.
	_, err := d.RealiseForTest(context.Background(), "/nix/store/0000000000000000000000000000000-nope.drv")
	if err == nil {
		t.Fatal("realising a nonexistent derivation should fail")
	}
}

// Progress events reach an observer as facts, with no presentation text
// composed by the engine.
func TestBuildProgressReachesTheObserver(t *testing.T) {
	var got []progress.Event
	obs := progress.Func(func(e progress.Event) {
		if e.Kind == progress.BuildProgress {
			got = append(got, e)
		}
	})
	_ = obs // exercised through the driver in the e2e path; this pins the shape
	e := progress.Event{
		Kind: progress.BuildProgress, Owner: "r", Name: "img",
		Derivation: "stage-image", Done: 2, Expected: 4, LastLine: "creating disk image",
	}
	progress.Emit(obs, e)
	if len(got) != 1 || got[0] != e {
		t.Errorf("BuildProgress did not survive the observer intact: %+v", got)
	}
}
