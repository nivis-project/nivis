// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/nivis-project/nivis/internal/progress"
)

// redrawInterval is how often the live region repaints. It repaints on a TICKER
// reading a snapshot of state, never once per event: events are cheap and
// arrive in bursts (a provider emitting notes, and — once Nix's own events are
// re-rendered — a trivial build produces a dozen progress results), so a
// per-event redraw would flicker and waste the terminal's time. A ticker also
// means an event storm cannot outrun the display.
const redrawInterval = 100 * time.Millisecond

// live keeps completed work in the terminal's own scrollback and shows what is
// currently in flight in a small region below it, rewritten in place and erased
// before anything else is printed.
type live struct {
	base

	// inFlight is a SET, not a single item, even though the engines resolve one
	// node at a time today. Modelling N costs nothing now and means parallel
	// apply later is a layout change rather than a rewrite.
	inFlight map[string]inFlightItem
	// order preserves arrival order so the region does not reshuffle between
	// redraws.
	order []string

	phase, phaseCount, phaseDone int

	// rows is how many lines the region currently occupies, so it can be erased
	// exactly. Getting this wrong is what corrupts a display.
	rows int
	// suspended is set while a subprocess owns the terminal.
	suspended bool
	closed    bool

	stop chan struct{}
	done chan struct{}

	frame int
}

type inFlightItem struct {
	label   string
	started time.Time
	// detail is a second, subordinate row: for a build, its derivation count
	// and the latest line of its own output. Empty means no second row.
	detail string
}

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func newLive(w io.Writer, level Level, color bool) *live {
	l := &live{
		base:     base{w: w, level: level, color: color},
		inFlight: map[string]inFlightItem{},
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go l.loop()
	return l
}

func (l *live) loop() {
	defer close(l.done)
	t := time.NewTicker(redrawInterval)
	defer t.Stop()
	for {
		select {
		case <-l.stop:
			return
		case <-t.C:
			l.mu.Lock()
			l.frame++
			l.redraw()
			l.mu.Unlock()
		}
	}
}

func (l *live) Emit(e progress.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Track in-flight work first, so the line committed below is drawn against
	// up-to-date state.
	switch e.Kind {
	case progress.NodeStart:
		l.add(e.ID, e.ID)
	case progress.NodeDone:
		l.remove(e.ID)
		if !e.Refresh {
			l.phaseDone++
		}
	case progress.BuildStart:
		l.add("build:"+e.Path, "building "+e.Name)
	case progress.BuildProgress:
		l.setDetail("build:"+e.Path, buildDetail(e))
	case progress.BuildDone:
		l.remove("build:" + e.Path)
	case progress.EvalStart:
		l.add("eval", fmt.Sprintf("evaluating configuration (phase %d)", e.Phase+1))
	case progress.EvalDone:
		l.remove("eval")
	case progress.ProviderSpawn:
		// Transient and usually brief; shown only as it happens.
	case progress.PhaseStart:
		l.phase, l.phaseCount, l.phaseDone = e.Phase, e.Count, 0
	}

	// Only a COMMITTED line repaints immediately, because it must erase the
	// region, write above it and repaint beneath. A silent event changes only
	// the region's contents, and the ticker will pick that up within one frame
	// — repainting here instead would make the redraw rate track the event
	// rate, which is exactly what the ticker exists to prevent.
	if line := l.line(e); line != "" {
		l.commit(line)
	}
}

func (l *live) add(key, label string) {
	if _, ok := l.inFlight[key]; !ok {
		l.order = append(l.order, key)
	}
	l.inFlight[key] = inFlightItem{label: label, started: time.Now()}
}

// setDetail updates an in-flight item's subordinate row, leaving its label and
// start time alone.
func (l *live) setDetail(key, detail string) {
	it, ok := l.inFlight[key]
	if !ok {
		return
	}
	it.detail = detail
	l.inFlight[key] = it
}

// buildDetail is the second row for a running build: what is building now, how
// far along the whole realise is, and its latest output line.
//
// The count is rendered as done/expected even though EXPECTED MOVES — Nix
// discovers work as it proceeds. Showing the current figure is honest;
// smoothing it would invent certainty Nix does not have.
func buildDetail(e progress.Event) string {
	var parts []string
	if e.Derivation != "" {
		parts = append(parts, e.Derivation)
	}
	if e.Expected > 0 {
		parts = append(parts, fmt.Sprintf("[%d/%d drv]", e.Done, e.Expected))
	}
	head := strings.Join(parts, "  ")
	if e.LastLine == "" {
		return head
	}
	if head == "" {
		return e.LastLine
	}
	return head + "\n" + e.LastLine
}

func (l *live) remove(key string) {
	if _, ok := l.inFlight[key]; !ok {
		return
	}
	delete(l.inFlight, key)
	for i, k := range l.order {
		if k == key {
			l.order = append(l.order[:i], l.order[i+1:]...)
			break
		}
	}
}

// commit writes a line into the terminal's own scrollback, above the region.
// The region is erased first and redrawn after, so the line lands cleanly.
// Caller holds the lock.
func (l *live) commit(line string) {
	l.erase()
	fmt.Fprintln(l.w, line)
	l.redraw()
}

// erase removes the region, leaving the cursor where it began. Caller holds the
// lock.
func (l *live) erase() {
	if l.rows == 0 {
		return
	}
	// Move up over each row, clearing it.
	for i := 0; i < l.rows; i++ {
		fmt.Fprint(l.w, "\x1b[1A\x1b[2K\r")
	}
	l.rows = 0
}

// redraw paints the region from current state. Caller holds the lock.
func (l *live) redraw() {
	l.erase()
	if l.closed || l.suspended || len(l.order) == 0 {
		return
	}
	frames := spinner[l.frame%len(spinner)]
	for _, key := range l.order {
		it := l.inFlight[key]
		label := truncate(it.label, l.width()-24)
		row := fmt.Sprintf("%s %s %s",
			l.paint(Ember, frames), label, l.paint(Dim, dur(time.Since(it.started))))
		if l.phaseCount > 0 && key != "eval" {
			row += l.paint(Dim, fmt.Sprintf("  [%d/%d]", l.phaseDone+1, l.phaseCount))
		}
		fmt.Fprintln(l.w, row)
		l.rows++

		// The subordinate row(s): a build's derivation count and its latest
		// output line. Truncated like the label — nixpkgs output runs long,
		// and a wrapped line would make the region grow and jump.
		for _, d := range strings.Split(it.detail, "\n") {
			if d == "" {
				continue
			}
			fmt.Fprintln(l.w, "    "+l.paint(Dim, truncate(d, l.width()-8)))
			l.rows++
		}
	}
}

// width is the terminal width to elide long labels against, with a conservative
// default when it cannot be determined.
func (l *live) width() int {
	if w := terminalWidth(l.w); w > 0 {
		return w
	}
	return 80
}

// Writer hands another producer a writer that erases the region, writes, and
// repaints — so a provider note appears as a complete line with the region
// intact beneath it, instead of landing in the middle of a redraw.
func (l *live) Writer() io.Writer { return &liveWriter{l: l} }

type liveWriter struct{ l *live }

func (w *liveWriter) Write(b []byte) (int, error) {
	w.l.mu.Lock()
	defer w.l.mu.Unlock()
	w.l.erase()
	n, err := w.l.w.Write(b)
	w.l.redraw()
	return n, err
}

// Suspend erases the region and holds it down for the duration of fn, so a
// subprocess can own the cursor. Without this, Nix's own progress display and
// this region fight over the same lines and corrupt each other.
func (l *live) Suspend(fn func() error) error {
	l.mu.Lock()
	l.erase()
	l.suspended = true
	l.mu.Unlock()

	err := fn()

	l.mu.Lock()
	l.suspended = false
	l.redraw()
	l.mu.Unlock()
	return err
}

// Close stops the repaint loop and clears the region. It must run before the
// result or an error is printed, or that output lands on top of a spinner. Safe
// to call twice.
func (l *live) Close() {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	l.closed = true
	l.mu.Unlock()

	close(l.stop)
	<-l.done

	l.mu.Lock()
	l.erase()
	l.mu.Unlock()
}
