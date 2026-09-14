// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package nixlog_test

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/nixlog"
)

// replay feeds a whole stream to a decoder and returns every distinct snapshot
// it produced, in order.
func replay(t *testing.T, lines []string) (*nixlog.Decoder, []nixlog.Update) {
	t.Helper()
	d := nixlog.New()
	var seen []nixlog.Update
	for _, ln := range lines {
		if changed, _ := d.Line(ln); changed {
			seen = append(seen, d.Update())
		}
	}
	return d, seen
}

// minFixtures is the floor on how many captured streams must be present.
//
// The fixtures are DISCOVERED by glob, so the set widens on its own when the
// pinned nixpkgs offers more versions. The floor is what stops that
// degenerating into a test that passes on zero inputs: a glob-driven check with
// no minimum is green in an empty directory, which is the failure shape
// nixform2-p72l and nixform2-pex6 are both open for. Raise it when the pin
// offers more.
const minFixtures = 4

// fixturePaths lists every captured stream on disk.
func fixturePaths(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob("testdata/realise-*.jsonl")
	if err != nil {
		t.Fatalf("globbing fixtures: %v", err)
	}
	if len(paths) < minFixtures {
		t.Fatalf("found %d fixture(s) (%v), want at least %d. "+
			"Regenerate with internal/nixlog/testdata/capture.sh — a decoder verified "+
			"against one nix is verified against one machine.",
			len(paths), paths, minFixtures)
	}
	sort.Strings(paths)
	return paths
}

// readLines loads one captured stream.
func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("fixture %s: %v", path, err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 1<<20)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("fixture %s: %v", path, err)
	}
	return lines
}

// fixtureVersion reads the nix version a capture records. Provenance matters:
// after a pin bump a STALE capture and a BROKEN decoder produce the same red
// test, and this header is what separates them.
func fixtureVersion(lines []string) string {
	for _, ln := range lines {
		if v, ok := strings.CutPrefix(ln, "# nix version:"); ok {
			return strings.TrimSpace(v)
		}
	}
	return "unrecorded"
}

// fixture loads one captured stream, for the tests that need only a
// representative one. It is a REAL capture from nix (see testdata/capture.sh),
// replayed so the decoder is tested without a nix binary in the test path.
func fixture(t *testing.T) []string {
	t.Helper()
	return readLines(t, fixturePaths(t)[0])
}

// THE trap this decoder exists to avoid: the same type value means different
// things under different actions. 105 is a build starting, or a progress
// report. Nothing in the payload distinguishes them.
func TestActionAndTypeTogetherDecideMeaning(t *testing.T) {
	d := nixlog.New()
	// Establish the builds aggregate so progress is attributed.
	d.Line(`@nix {"action":"start","id":1,"type":104}`)

	// type 105 under `start` = a build begins.
	d.Line(`@nix {"action":"start","id":2,"type":105,"fields":["/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-img.drv"]}`)
	if got := d.Update().Building; got != "img" {
		t.Errorf("start/105 should name the derivation building; got %q", got)
	}

	// type 105 under `result` = progress, NOT another build.
	d.Line(`@nix {"action":"result","id":1,"type":105,"fields":[2,7,0,0]}`)
	u := d.Update()
	if u.Building != "img" {
		t.Errorf("result/105 must not be read as a build start; Building = %q", u.Building)
	}
	if u.Done != 2 || u.Expected != 7 {
		t.Errorf("result/105 should be progress; got %d/%d", u.Done, u.Expected)
	}

	// type 101 under `result` = a build log line, not a download.
	d.Line(`@nix {"action":"result","id":2,"type":101,"fields":["compiling foo.c"]}`)
	if got := d.Update().LastLine; got != "compiling foo.c" {
		t.Errorf("result/101 should be a build output line; got %q", got)
	}
}

// Progress belongs to an activity. A substituter query completing "3 of 3"
// before any build starts is a REAL observed case; read as build progress it
// announces the build finished before it began.
func TestOtherActivitiesProgressIsNotBuildProgress(t *testing.T) {
	d := nixlog.New()
	// A file transfer reports its own completed progress...
	d.Line(`@nix {"action":"start","id":9,"type":101,"fields":["https://cache.nixos.org/x.narinfo"]}`)
	d.Line(`@nix {"action":"result","id":9,"type":105,"fields":[3,3,0,0]}`)

	if u := d.Update(); u.Done != 0 || u.Expected != 0 {
		t.Fatalf("a file transfer's progress was read as build progress: %d/%d", u.Done, u.Expected)
	}

	// ...and only the builds aggregate sets the real counts.
	d.Line(`@nix {"action":"start","id":1,"type":104}`)
	d.Line(`@nix {"action":"result","id":1,"type":105,"fields":[0,3,1,0]}`)
	if u := d.Update(); u.Done != 0 || u.Expected != 3 {
		t.Errorf("build progress = %d/%d, want 0/3", u.Done, u.Expected)
	}
}

// The expected total MOVES: Nix discovers work as it goes. Reporting the first
// figure as final would be a lie the stream does not support.
func TestExpectedTotalFollowsTheStream(t *testing.T) {
	_, seen := replay(t, fixture(t))

	var totals []int
	for _, u := range seen {
		if len(totals) == 0 || totals[len(totals)-1] != u.Expected {
			totals = append(totals, u.Expected)
		}
	}
	if len(totals) < 3 {
		t.Fatalf("the fixture should exercise a MOVING expected total; saw %v. "+
			"Was it captured from a single-derivation build?", totals)
	}
	last := seen[len(seen)-1]
	if last.Expected < 3 {
		t.Errorf("final expected = %d, want at least the three derivations built", last.Expected)
	}
	if last.Done == 0 {
		t.Error("final done = 0; no build was ever counted as finished")
	}
}

// The fixture is a real build of three derivations: each should be named as it
// starts, and its own output should come through.
func TestFixtureReportsEachBuildAndItsOutput(t *testing.T) {
	_, seen := replay(t, fixture(t))

	names := map[string]bool{}
	lines := map[string]bool{}
	for _, u := range seen {
		if u.Building != "" {
			names[u.Building] = true
		}
		if u.LastLine != "" {
			lines[u.LastLine] = true
		}
	}

	for _, want := range []string{"nivis-stage-kernel-TOKEN", "nivis-stage-initrd-TOKEN", "nivis-stage-image-TOKEN"} {
		if !names[want] {
			t.Errorf("the fixture should report %q building; saw %v", want, keys(names))
		}
	}
	// The builder printed these; they must arrive as build output, which is
	// what proves --print-build-logs is not needed.
	for _, want := range []string{"unpacking sources", "creating disk image (2048 MiB)", "copying closure"} {
		if !lines[want] {
			t.Errorf("the fixture should carry the build's own line %q", want)
		}
	}
}

// Degrade, never fail. None of these may error, panic, or stop the decoder.
func TestMalformedAndUnknownInputIsIgnored(t *testing.T) {
	d := nixlog.New()
	d.Line(`@nix {"action":"start","id":1,"type":104}`)
	d.Line(`@nix {"action":"result","id":1,"type":105,"fields":[1,4,0,0]}`)
	before := d.Update()

	for _, bad := range []string{
		`@nix {"action":"start","id":2,"type":99999}`,                   // unknown activity type
		`@nix {"action":"result","id":1,"type":99999,"fields":[]}`,      // unknown result type
		`@nix {"action":"teleport","id":3}`,                             // unknown action
		`@nix {not json at all`,                                         // malformed
		`@nix {"action":"start","id":4}`,                                // no type
		`@nix {"action":"result","id":1,"type":105}`,                    // no fields
		`@nix {"action":"result","id":1,"type":105,"fields":["a","b"]}`, // wrong field types
		`@nix `, // empty payload
		`ordinary build output, not part of the stream`, // no prefix
		``, // empty line
	} {
		d.Line(bad) // must not panic
	}

	if got := d.Update(); got != before {
		t.Errorf("malformed or unknown input changed the snapshot:\n before %+v\n after  %+v", before, got)
	}

	// And the decoder still works afterwards.
	d.Line(`@nix {"action":"result","id":1,"type":105,"fields":[2,4,0,0]}`)
	if got := d.Update(); got.Done != 2 {
		t.Errorf("the decoder stopped working after bad input; done = %d, want 2", got.Done)
	}
}

// A stream cut off mid-event must not wedge the decoder.
func TestTruncatedStreamIsSafe(t *testing.T) {
	lines := fixture(t)
	// Cut the last line in half.
	cut := lines[:len(lines)-1]
	last := lines[len(lines)-1]
	cut = append(cut, last[:len(last)/2])

	d, _ := replay(t, cut)
	_ = d.Update() // must simply be whatever it had; no panic, no error
}

func TestDerivationName(t *testing.T) {
	cases := map[string]string{
		"/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-nixos-image.drv": "nixos-image",
		"/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-nixos-image":     "nixos-image",
		"/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-a-b-c.drv":       "a-b-c",
		"/tmp/not-a-store-path.drv":                                   "not-a-store-path",
		"plain":                                                       "plain",
		"":                                                            ".",
	}
	for in, want := range cases {
		if got := nixlog.DerivationName(in); got != want {
			t.Errorf("DerivationName(%q) = %q, want %q", in, got, want)
		}
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// The distinction that matters most in practice: a well-formed event that
// changes nothing is UNDERSTOOD. Most of a real stream is exactly that — one
// narinfo query emits a dozen identical progress tuples — and a caller that
// reads "unchanged" as "unreadable" gives up on the format within the first
// activity and never reports a build, silently, because falling back is not an
// error.
func TestUnchangedIsStillUnderstood(t *testing.T) {
	d := nixlog.New()
	d.Line(`@nix {"action":"start","id":1,"type":104}`)

	// The same tuple twice: the second changes nothing but is perfectly readable.
	if changed, understood := d.Line(`@nix {"action":"result","id":1,"type":105,"fields":[1,4,0,0]}`); !changed || !understood {
		t.Errorf("first progress: changed=%v understood=%v, want true/true", changed, understood)
	}
	if changed, understood := d.Line(`@nix {"action":"result","id":1,"type":105,"fields":[1,4,0,0]}`); changed || !understood {
		t.Errorf("repeated progress: changed=%v understood=%v, want false/true", changed, understood)
	}

	// An activity we do not act on is still understood — it is part of the stream.
	if _, understood := d.Line(`@nix {"action":"start","id":5,"type":109,"fields":["/nix/store/x","cache"]}`); !understood {
		t.Error("a known action with an activity type we ignore is still part of the stream")
	}
	// A stop is understood too.
	if _, understood := d.Line(`@nix {"action":"stop","id":5}`); !understood {
		t.Error("a stop is part of the stream")
	}
}

// Only genuinely unreadable input reports understood=false: that is the signal
// a caller uses to decide the format is no longer one it knows.
func TestUnreadableInputReportsNotUnderstood(t *testing.T) {
	d := nixlog.New()
	for _, bad := range []string{
		`@nix {not json`,                    // malformed
		`@nix {"action":"teleport","id":3}`, // an action we do not know
		`ordinary build output`,             // not part of the stream
		``,                                  // empty
	} {
		if _, understood := d.Line(bad); understood {
			t.Errorf("%q should not be reported as understood", bad)
		}
	}
}

// A realistic run of unchanging events must NOT look like a broken format.
// This is the regression guard for the bug an integration test caught: the
// streaming layer degraded to raw output before the build began, because it
// counted "unchanged" as "unreadable".
func TestALongRunOfUnchangingEventsStaysUnderstood(t *testing.T) {
	d := nixlog.New()
	d.Line(`@nix {"action":"start","id":9,"type":101,"fields":["https://cache.nixos.org/x.narinfo"]}`)
	for i := 0; i < 50; i++ {
		if _, understood := d.Line(`@nix {"action":"result","id":9,"type":105,"fields":[0,0,0,0]}`); !understood {
			t.Fatalf("event %d of an ordinary narinfo query was reported as unreadable", i)
		}
	}
}
