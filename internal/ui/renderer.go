// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/progress"
)

// Renderer presents a run. It is an Observer (the engines report to it) and it
// is the SOLE OWNER of the stream it writes to: anything else with something to
// print must go through Writer or Suspend.
type Renderer interface {
	progress.Observer

	// Writer returns the io.Writer other producers must use instead of writing
	// to the stream directly — today the provider-log sink. Writes through it
	// are serialised against the renderer's own output and, for a live
	// renderer, do not corrupt a region mid-redraw.
	Writer() io.Writer

	// Suspend releases the terminal for the duration of fn, so a subprocess can
	// own the cursor, then restores the renderer's display. Use it around
	// anything that writes to the terminal itself.
	Suspend(fn func() error) error

	// Close tears down any live display and clears its space. It must be called
	// before the result or an error is printed, and is safe to call twice.
	Close()
}

// New builds the renderer for w at the given level and colour setting. It picks
// the live renderer only when the terminal is positively known to handle cursor
// control; anything else gets the plain one, which carries the same information
// one line per transition.
func New(w io.Writer, level Level, mode ColorMode) Renderer {
	color := ColorEnabled(w, mode)
	if CursorEnabled(w, level) {
		return newLive(w, level, color)
	}
	return &plain{base: base{w: w, level: level, color: color}}
}

// base is the state and formatting both renderers share.
type base struct {
	w     io.Writer
	level Level
	color bool
	mu    sync.Mutex
}

func (b *base) paint(code, s string) string {
	if !b.color {
		return s
	}
	return code + s + Reset
}

// marker is the change-type marker for a node, matching the plan/apply legend:
// `+` create, `~` update, `-/+` replace, `=` no-op, `r` a datasource read.
func (b *base) marker(e progress.Event) string {
	if e.IsData {
		return b.paint(Dim, "r")
	}
	switch e.Op {
	case plan.OpUpdate:
		return b.paint(Yellow, "~")
	case plan.OpReplace:
		return b.paint(Red, "-") + b.paint(Green, "/+")
	case plan.OpNoop:
		return b.paint(Dim, "=")
	default:
		return b.paint(Green, "+")
	}
}

// dur renders a duration the way a reader scans it: whole seconds once past a
// second, milliseconds below that.
func dur(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// line formats one committed scrollback line for an event, or "" when the event
// says nothing at this level. This is the single place wording is decided, so
// the plain and live renderers cannot drift apart.
func (b *base) line(e progress.Event) string {
	switch e.Kind {
	case progress.EvalStart:
		if b.level >= LevelVerbose {
			return b.paint(Dim, fmt.Sprintf("evaluating configuration (phase %d)", e.Phase))
		}
	case progress.EvalDone:
		if e.Err != nil {
			return "" // the error itself is reported by the command
		}
		if b.level >= LevelVerbose {
			return b.paint(Dim, fmt.Sprintf("evaluated configuration (phase %d) %s", e.Phase, dur(e.Duration)))
		}
	case progress.PhaseStart:
		if b.level >= LevelInfo {
			return b.paint(Ice, fmt.Sprintf("Phase %d", e.Phase+1)) +
				b.paint(Dim, fmt.Sprintf("  %s", plural(e.Count, "node", "nodes")))
		}
	case progress.NodeStart:
		if b.level >= LevelVerbose {
			return fmt.Sprintf("  %s %s %s", b.marker(e), e.ID, b.paint(Dim, "..."))
		}
	case progress.NodeDone:
		if e.Err != nil {
			return "" // the error itself is reported by the command
		}
		return b.nodeDoneLine(e)
	case progress.BuildStart:
		if b.level >= LevelInfo {
			return b.paint(Dim, fmt.Sprintf("  building %s for %s", e.Name, e.Owner))
		}
	case progress.BuildDone:
		if e.Err != nil {
			return ""
		}
		if b.level >= LevelInfo {
			return b.paint(Dim, fmt.Sprintf("  built %s %s", e.Name, dur(e.Duration)))
		}
	case progress.ProviderSpawn:
		if b.level >= LevelVerbose {
			return b.paint(Dim, fmt.Sprintf("  starting provider %s", e.Name))
		}
	case progress.Note:
		return e.Message
	}
	return ""
}

// nodeDoneLine renders a completed node. At the default level a refresh that
// found NO drift is not named: reporting forty unchanged reads buries the three
// that changed, so the default reports what CHANGED and a higher level reports
// what happened.
func (b *base) nodeDoneLine(e progress.Event) string {
	if e.Drifted {
		return fmt.Sprintf("  %s %s %s", b.paint(Yellow, "~"), e.ID, b.paint(Yellow, "drifted"))
	}
	if e.Refresh && b.level < LevelVerbose {
		return ""
	}
	if b.level < LevelInfo {
		return ""
	}
	s := fmt.Sprintf("  %s %s", b.marker(e), e.ID)
	if d := dur(e.Duration); d != "" {
		s += b.paint(Dim, "  "+d)
	}
	if e.ResourceID != "" {
		s += b.paint(Dim, "  "+e.ResourceID)
	}
	return s
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// truncate elides the middle of s so it fits width, keeping both ends. Long
// resource ids are informative at both ends — the provider and type at the
// front, the instance name at the back — so a middle elision beats a tail cut,
// and beats the wrap that makes an unbounded line unreadable.
func truncate(s string, width int) string {
	if width <= 0 || len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	keep := width - 1 // room for the ellipsis
	head := (keep + 1) / 2
	tail := keep - head
	return s[:head] + "…" + s[len(s)-tail:]
}
