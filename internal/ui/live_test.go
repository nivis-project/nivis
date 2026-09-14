// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nivis-project/nivis/internal/progress"
)

// fakeTerm is a writer that records everything, standing in for a terminal.
// It is safe for the repaint goroutine to write to while a test reads.
type fakeTerm struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (f *fakeTerm) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.Write(b)
}

func (f *fakeTerm) String() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.buf.String()
}

// newTestLive builds a live renderer over a fake terminal. Colour is off so
// assertions read plainly; the cursor sequences are still emitted, since those
// are what is under test.
func newTestLive(t *testing.T) (*live, *fakeTerm) {
	t.Helper()
	ft := &fakeTerm{}
	l := newLive(ft, LevelInfo, false)
	t.Cleanup(l.Close)
	return l, ft
}

// Completed work goes into the terminal's own scrollback, so it survives the
// region being erased and repainted.
func TestLiveCommitsCompletedWorkToScrollback(t *testing.T) {
	l, ft := newTestLive(t)

	l.Emit(progress.Event{Kind: progress.PhaseStart, Phase: 0, Count: 2})
	l.Emit(progress.Event{Kind: progress.NodeStart, ID: "alpha.a"})
	l.Emit(progress.Event{Kind: progress.NodeDone, ID: "alpha.a", Duration: time.Second})
	l.Close()

	got := stripANSI(ft.String())
	if !strings.Contains(got, "alpha.a") {
		t.Errorf("a completed node should be committed to scrollback; got:\n%q", got)
	}
	if !strings.Contains(got, "Phase 1") {
		t.Errorf("the phase heading should be committed; got:\n%q", got)
	}
}

// The region is erased when the renderer closes, so the result that follows is
// not printed on top of a spinner.
func TestLiveClosesWithNoResidualRegion(t *testing.T) {
	l, ft := newTestLive(t)

	l.Emit(progress.Event{Kind: progress.NodeStart, ID: "alpha.slow"})
	// Let the ticker paint the region at least once; painting is deliberately
	// NOT synchronous with the event (see TestLiveRedrawIsBoundedByTime...).
	waitFor(t, func() bool {
		l.mu.Lock()
		defer l.mu.Unlock()
		return l.rows > 0
	}, "the region to be painted")

	l.Close()

	// After Close the region is gone: its lines are cleared and rows is zero.
	if l.rows != 0 {
		t.Errorf("rows = %d after Close, want 0 — the region was not erased", l.rows)
	}
	if !strings.Contains(ft.String(), "\x1b[2K") {
		t.Error("Close should have erased the region with a clear-line sequence")
	}
}

// Close is safe to call twice: the commands call it explicitly before printing
// the result AND defer it against an early return.
func TestLiveCloseIsIdempotent(t *testing.T) {
	l, _ := newTestLive(t)
	l.Close()
	l.Close() // must not panic or deadlock
}

// The region repaints on a TICKER reading a snapshot, not once per event. An
// event storm must not turn into a redraw storm: provider notes arrive in
// bursts, and re-rendered Nix events will arrive in far larger ones — a trivial
// build emits over a dozen progress results.
func TestLiveRedrawIsBoundedByTimeNotEventCount(t *testing.T) {
	l, ft := newTestLive(t)

	// One in-flight item so there is something to paint.
	l.Emit(progress.Event{Kind: progress.NodeStart, ID: "alpha.slow"})

	// A burst of events that render no committed line (a spawn at info level is
	// silent), so every write observed is a repaint.
	const burst = 500
	for i := 0; i < burst; i++ {
		l.Emit(progress.Event{Kind: progress.ProviderSpawn, Name: "alpha"})
	}
	l.Close()

	repaints := strings.Count(ft.String(), "\x1b[2K")
	if repaints >= burst {
		t.Errorf("repaints = %d for %d events; the region must not repaint per event",
			repaints, burst)
	}
}

// In-flight work is modelled as a SET, not a single item. The engines resolve
// one node at a time today, but modelling N costs nothing now and means
// parallel apply later is a layout change rather than a rewrite.
func TestLiveHandlesConcurrentInFlightItems(t *testing.T) {
	l, _ := newTestLive(t)

	l.Emit(progress.Event{Kind: progress.NodeStart, ID: "alpha.a"})
	l.Emit(progress.Event{Kind: progress.NodeStart, ID: "alpha.b"})

	l.mu.Lock()
	n := len(l.inFlight)
	order := append([]string(nil), l.order...)
	l.mu.Unlock()

	if n != 2 {
		t.Fatalf("in-flight = %d, want 2", n)
	}
	if order[0] != "alpha.a" || order[1] != "alpha.b" {
		t.Errorf("arrival order should be preserved so the region does not reshuffle; got %v", order)
	}

	l.Emit(progress.Event{Kind: progress.NodeDone, ID: "alpha.a"})
	l.mu.Lock()
	n = len(l.inFlight)
	l.mu.Unlock()
	if n != 1 {
		t.Errorf("in-flight = %d after one completed, want 1", n)
	}
}

// Suspend holds the region down for the duration of fn, so a subprocess can own
// the cursor. Without this, Nix's own progress display and this region fight
// over the same lines.
func TestLiveSuspendReleasesTheTerminal(t *testing.T) {
	l, _ := newTestLive(t)
	l.Emit(progress.Event{Kind: progress.NodeStart, ID: "alpha.a"})

	var suspendedDuring bool
	err := l.Suspend(func() error {
		l.mu.Lock()
		suspendedDuring = l.suspended && l.rows == 0
		l.mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !suspendedDuring {
		t.Error("the region must be erased and held down for the subprocess's duration")
	}

	l.mu.Lock()
	stillSuspended := l.suspended
	l.mu.Unlock()
	if stillSuspended {
		t.Error("the region must be restored after the subprocess")
	}
}

// Suspend propagates the function's error rather than swallowing it.
func TestLiveSuspendPropagatesError(t *testing.T) {
	l, _ := newTestLive(t)
	want := errTest
	if got := l.Suspend(func() error { return want }); got != want {
		t.Errorf("Suspend returned %v, want %v", got, want)
	}
}

// A note written through the renderer's writer appears as a complete line, with
// the region repainted beneath it — not interleaved into a half-drawn frame.
func TestLiveWriterDoesNotCorruptTheRegion(t *testing.T) {
	l, ft := newTestLive(t)
	l.Emit(progress.Event{Kind: progress.NodeStart, ID: "alpha.a"})

	if _, err := l.Writer().Write([]byte("warning: from a provider\n")); err != nil {
		t.Fatal(err)
	}
	l.Close()

	got := stripANSI(ft.String())
	if !strings.Contains(got, "warning: from a provider") {
		t.Errorf("the note should appear; got:\n%q", got)
	}
	// The note must be on a line of its own, not glued to a spinner frame.
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "warning: from a provider") && strings.Contains(line, "alpha.a") {
			t.Errorf("the note was interleaved with the region: %q", line)
		}
	}
}

// A long label is elided in the MIDDLE rather than wrapped. Resource ids are
// informative at both ends — the provider and type at the front, the instance
// name at the back — and an unbounded line wrapping mid-token is the main
// reason a long transcript becomes unreadable.
func TestTruncateElidesTheMiddle(t *testing.T) {
	const id = "module.instance.aws_security_group_rule.ec2nix_security_group_ingress_rule"
	got := truncate(id, 40)
	if len([]rune(got)) > 40 {
		t.Errorf("truncate returned %d runes, want at most 40: %q", len([]rune(got)), got)
	}
	if !strings.HasPrefix(got, "module.instance") {
		t.Errorf("the head should survive; got %q", got)
	}
	if !strings.HasSuffix(got, "rule") {
		t.Errorf("the tail should survive; got %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("an elision should be marked; got %q", got)
	}
	// A string that fits is returned unchanged.
	if got := truncate("short", 40); got != "short" {
		t.Errorf("a fitting string must be unchanged; got %q", got)
	}
}

func TestDurFormatting(t *testing.T) {
	cases := map[time.Duration]string{
		0:                             "",
		250 * time.Millisecond:        "250ms",
		time.Second:                   "1s",
		42 * time.Second:              "42s",
		2*time.Minute + 4*time.Second: "2m04s",
	}
	for d, want := range cases {
		if got := dur(d); got != want {
			t.Errorf("dur(%v) = %q, want %q", d, got, want)
		}
	}
}

// waitFor polls cond until it holds, failing the test if it never does. The
// live renderer paints on a ticker, so a test that needs a painted region must
// wait for a frame rather than assume one.
func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// A failure mid-run must find the region already gone. The command prints its
// `error:` line after Close, and if the region were still on screen the error
// would land on top of a spinner.
func TestLiveRegionIsGoneBeforeAnErrorIsPrinted(t *testing.T) {
	l, ft := newTestLive(t)

	l.Emit(progress.Event{Kind: progress.NodeStart, ID: "alpha.failing"})
	waitFor(t, func() bool {
		l.mu.Lock()
		defer l.mu.Unlock()
		return l.rows > 0
	}, "the region to be painted")

	// What a command does on the error path: Close, then print.
	l.Close()
	before := ft.String()
	if _, err := ft.Write([]byte("error: apply failed\n")); err != nil {
		t.Fatal(err)
	}

	// Everything written after Close is the error alone — no repaint follows it.
	after := strings.TrimPrefix(ft.String(), before)
	if after != "error: apply failed\n" {
		t.Errorf("output after Close should be the error alone; got %q", after)
	}
	if l.rows != 0 {
		t.Errorf("rows = %d, want 0: the region must be erased before the error", l.rows)
	}
}

// Emitting after Close must not resurrect the region or panic: a deferred
// Close runs after an explicit one, and an engine goroutine may still be
// unwinding.
func TestLiveEmitAfterCloseIsInert(t *testing.T) {
	l, ft := newTestLive(t)
	l.Close()
	before := ft.String()

	l.Emit(progress.Event{Kind: progress.NodeStart, ID: "alpha.late"})

	if l.rows != 0 {
		t.Errorf("rows = %d after emitting post-Close, want 0", l.rows)
	}
	if got := strings.TrimPrefix(ft.String(), before); strings.Contains(got, "\x1b") {
		t.Errorf("no cursor control should follow Close; got %q", got)
	}
}

// A running build shows what it is building, how far along the whole realise
// is, and its latest output line — the four things that tell you a long build
// is alive rather than wedged.
func TestLiveShowsBuildProgress(t *testing.T) {
	l, ft := newTestLive(t)

	l.Emit(progress.Event{Kind: progress.NodeStart, ID: "aws.aws_s3_object.image"})
	l.Emit(progress.Event{
		Kind: progress.BuildStart, Owner: "aws.aws_s3_object.image",
		Name: "nixos-image", Path: "/nix/store/h-nixos-image",
	})
	l.Emit(progress.Event{
		Kind: progress.BuildProgress, Owner: "aws.aws_s3_object.image",
		Name: "nixos-image", Path: "/nix/store/h-nixos-image",
		Derivation: "stage-image", Done: 2, Expected: 4,
		LastLine: "creating disk image (2048 MiB)",
	})
	// Wait for the CONTENT, not merely for a painted region: an earlier
	// committed line already leaves rows > 0, so waiting on that would race
	// ahead of the tick that paints the build's detail.
	waitFor(t, func() bool {
		return strings.Contains(stripANSI(ft.String()), "creating disk image")
	}, "the build detail to be painted")
	l.Close()

	got := stripANSI(ft.String())
	for _, want := range []string{"building nixos-image", "stage-image", "[2/4 drv]", "creating disk image (2048 MiB)"} {
		if !strings.Contains(got, want) {
			t.Errorf("the build row should carry %q; got:\n%q", want, got)
		}
	}
}

// The expected total MOVES as Nix discovers work. The display must follow it,
// not freeze on the first figure.
func TestLiveFollowsAMovingExpectedTotal(t *testing.T) {
	l, ft := newTestLive(t)
	l.Emit(progress.Event{Kind: progress.BuildStart, Owner: "r", Name: "img", Path: "/p"})
	for _, c := range []struct{ done, expected int }{{0, 1}, {0, 3}, {2, 4}} {
		l.Emit(progress.Event{
			Kind: progress.BuildProgress, Path: "/p",
			Derivation: "d", Done: c.done, Expected: c.expected,
		})
		waitFor(t, func() bool {
			return strings.Contains(stripANSI(ft.String()), fmt.Sprintf("[%d/%d drv]", c.done, c.expected))
		},
			fmt.Sprintf("the region to show [%d/%d drv]", c.done, c.expected))
	}
	l.Close()
}

// nixpkgs output lines run long. A line wider than the terminal must be
// truncated, not wrapped: a wrapped line makes the region grow and jump, and
// the renderer's model of how many rows it owns goes wrong.
func TestLiveTruncatesLongBuildOutput(t *testing.T) {
	l, ft := newTestLive(t)
	l.Emit(progress.Event{Kind: progress.BuildStart, Owner: "r", Name: "img", Path: "/p"})
	l.Emit(progress.Event{
		Kind: progress.BuildProgress, Path: "/p", Derivation: "d", Done: 1, Expected: 2,
		LastLine: strings.Repeat("x", 500),
	})
	waitFor(t, func() bool {
		return strings.Contains(stripANSI(ft.String()), "[1/2 drv]")
	}, "the build detail to be painted")

	l.mu.Lock()
	rows := l.rows
	l.mu.Unlock()
	l.Close()

	// The build's own row plus two subordinate rows: the count and the output
	// line. A wrap would push it past that.
	if rows > 3 {
		t.Errorf("region is %d rows; a long output line must be truncated, not wrapped", rows)
	}
	for _, line := range strings.Split(stripANSI(ft.String()), "\n") {
		if len([]rune(line)) > 200 {
			t.Errorf("a rendered line is %d runes; it was not truncated", len([]rune(line)))
		}
	}
}
