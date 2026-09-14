// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase

// The stream-consuming loop, tested WITHOUT nix.
//
// This matters beyond coverage: the dekking/coverage gate runs in a sandbox
// with no nix binary, so anything reachable only through a real subprocess goes
// unexercised there. The degradation rule below has already been wrong once —
// it counted "the snapshot did not change" as "the line could not be read" and
// so gave up on the format before the build began, silently. These tests are
// what keep that from coming back unnoticed in CI.

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/nixlog"
)

// consumeLines runs the loop over the given lines and reports what the user
// saw, what was captured, and every update produced.
func consumeLines(lines []string) (seen, captured string, updates []nixlog.Update) {
	var out, buf bytes.Buffer
	term := Terminal{
		Stderr:  &out,
		OnBuild: func(u nixlog.Update) { updates = append(updates, u) },
	}
	term.consume(strings.NewReader(strings.Join(lines, "\n")+"\n"), &buf)
	return out.String(), buf.String(), updates
}

// The real fixture, replayed through the loop rather than through nix.
func fixtureLines(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("../nixlog/testdata/realise-chain.jsonl")
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	var lines []string
	for _, ln := range strings.Split(string(b), "\n") {
		if ln != "" {
			lines = append(lines, ln)
		}
	}
	return lines
}

func TestConsumeReportsBuildsFromARealStream(t *testing.T) {
	seen, captured, updates := consumeLines(fixtureLines(t))

	if len(updates) == 0 {
		t.Fatal("a real captured stream produced no updates")
	}
	last := updates[len(updates)-1]
	if last.Expected < 3 || last.Done == 0 {
		t.Errorf("final update = %+v, want the three derivations counted", last)
	}
	// Structured lines are consumed, not shown: at this verbosity the live
	// region reports them instead.
	if strings.Contains(seen, "@nix ") {
		t.Errorf("structured lines should be consumed, not passed to the user:\n%s", seen)
	}
	// But everything is still captured, because the error path needs it all.
	if !strings.Contains(captured, "@nix ") {
		t.Error("the full output must be captured for the error path")
	}
}

// A long run of well-formed events that change nothing must NOT be mistaken
// for a broken format. This is the regression guard for the bug that shipped
// past every fixture test: a single narinfo query emits about fourteen
// identical progress tuples, which is enough to trip a naive counter.
func TestConsumeDoesNotDegradeOnUnchangingEvents(t *testing.T) {
	var lines []string
	lines = append(lines, `@nix {"action":"start","id":9,"type":101,"fields":["https://cache.nixos.org/x.narinfo"]}`)
	for i := 0; i < 3*maxUndecodable; i++ {
		lines = append(lines, `@nix {"action":"result","id":9,"type":105,"fields":[0,0,0,0]}`)
	}
	// ...and only then does the build start.
	lines = append(lines,
		`@nix {"action":"start","id":1,"type":104}`,
		`@nix {"action":"start","id":2,"type":105,"fields":["/nix/store/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-img.drv"]}`,
		`@nix {"action":"result","id":1,"type":105,"fields":[0,2,1,0]}`,
	)

	seen, _, updates := consumeLines(lines)

	if len(updates) == 0 {
		t.Fatal("the build was never reported: the loop degraded during the unchanging events")
	}
	if got := updates[len(updates)-1]; got.Building != "img" || got.Expected != 2 {
		t.Errorf("final update = %+v, want the build named and counted", got)
	}
	if strings.Contains(seen, "@nix ") {
		t.Errorf("the loop fell back to raw output although the stream was readable:\n%s", seen)
	}
}

// Output that is not part of the stream goes straight to the user. This is the
// continuous fallback: if Nix stops framing its output, it simply flows through.
func TestConsumePassesUnframedOutputThrough(t *testing.T) {
	seen, captured, _ := consumeLines([]string{
		`@nix {"action":"start","id":1,"type":104}`,
		`error: something nix wanted to say plainly`,
		`  and a second line of it`,
	})
	for _, want := range []string{"something nix wanted to say plainly", "and a second line of it"} {
		if !strings.Contains(seen, want) {
			t.Errorf("unframed output should reach the user; missing %q from:\n%s", want, seen)
		}
	}
	if !strings.Contains(captured, "error: something nix") {
		t.Error("unframed output must be captured too")
	}
}

// Enough consecutive UNREADABLE structured lines means the framing is not one
// we know: give up and pass everything through, rather than showing nothing.
func TestConsumeDegradesOnRepeatedUnreadableLines(t *testing.T) {
	var lines []string
	for i := 0; i < maxUndecodable+2; i++ {
		lines = append(lines, `@nix {"action":"from-the-future","id":1}`)
	}
	lines = append(lines, `@nix {"action":"start","id":1,"type":104}`)

	seen, _, _ := consumeLines(lines)
	if !strings.Contains(seen, "from-the-future") {
		t.Errorf("after repeated unreadable lines the loop should pass output through; got:\n%s", seen)
	}
	// Having degraded, it stays degraded — including for lines it could read.
	if !strings.Contains(seen, `"type":104`) {
		t.Error("once degraded the loop should pass everything through, not resume decoding")
	}
}

// With nowhere to write, passing through must still not panic or lose capture.
func TestConsumeWithoutAWriter(t *testing.T) {
	var buf bytes.Buffer
	term := Terminal{} // no Stderr, no OnBuild
	term.consume(strings.NewReader("plain line\n@nix {\"action\":\"start\",\"id\":1,\"type\":104}\n"), &buf)
	if !strings.Contains(buf.String(), "plain line") {
		t.Error("output must still be captured with no writer attached")
	}
}

// A stream cut mid-line is not an error: the scanner yields what it has.
func TestConsumeTruncatedStream(t *testing.T) {
	var buf bytes.Buffer
	term := Terminal{OnBuild: func(nixlog.Update) {}}
	term.consume(strings.NewReader(`@nix {"action":"start","id":1,"typ`), &buf)
	// No panic, and the partial line is still captured.
	if buf.Len() == 0 {
		t.Error("a truncated line should still be captured")
	}
}
