// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nivis-project/nivis/internal/plan"
	"github.com/nivis-project/nivis/internal/progress"
)

// render runs events through a plain renderer at a level and returns the text.
func render(level Level, color bool, events ...progress.Event) string {
	var buf bytes.Buffer
	r := &plain{base: base{w: &buf, level: level, color: color}}
	for _, e := range events {
		r.Emit(e)
	}
	return buf.String()
}

// Every change type gets its marker, matching the plan/apply legend.
func TestPlainMarkersByChangeType(t *testing.T) {
	cases := []struct {
		name   string
		event  progress.Event
		marker string
	}{
		{"create", progress.Event{Kind: progress.NodeDone, ID: "a", Op: plan.OpCreate}, "+"},
		{"update", progress.Event{Kind: progress.NodeDone, ID: "a", Op: plan.OpUpdate}, "~"},
		{"replace", progress.Event{Kind: progress.NodeDone, ID: "a", Op: plan.OpReplace}, "-/+"},
		{"noop", progress.Event{Kind: progress.NodeDone, ID: "a", Op: plan.OpNoop}, "="},
		{"datasource read", progress.Event{Kind: progress.NodeDone, ID: "a", IsData: true}, "r"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := render(LevelInfo, false, c.event)
			if !strings.Contains(got, c.marker+" a") {
				t.Errorf("want marker %q for %s; got %q", c.marker, c.name, got)
			}
		})
	}
}

// A destroy is reported through the same node events; the command's own result
// carries the destroy marker, so here the point is simply that it is reported.
func TestPlainReportsEachNode(t *testing.T) {
	got := render(LevelInfo, false,
		progress.Event{Kind: progress.NodeDone, ID: "alpha.a"},
		progress.Event{Kind: progress.NodeDone, ID: "alpha.b"},
	)
	if strings.Count(got, "\n") != 2 {
		t.Errorf("want one line per node; got %q", got)
	}
}

// Colour changes presentation only. With colour off there must be no escape
// codes at all; with it on, the same markers, text and counts must survive.
func TestColourChangesOnlyPresentation(t *testing.T) {
	events := []progress.Event{
		{Kind: progress.PhaseStart, Phase: 0, Count: 2},
		{Kind: progress.NodeDone, ID: "alpha.a", Op: plan.OpCreate, ResourceID: "id-1", Duration: time.Second},
		{Kind: progress.NodeDone, ID: "alpha.b", Op: plan.OpUpdate},
	}
	plainText := render(LevelInfo, false, events...)
	coloured := render(LevelInfo, true, events...)

	if strings.Contains(plainText, "\x1b[") {
		t.Errorf("colour off must emit no escape codes; got %q", plainText)
	}
	if !strings.Contains(coloured, "\x1b[") {
		t.Error("colour on emitted no escape codes")
	}
	if got := stripANSI(coloured); got != plainText {
		t.Errorf("stripping colour must yield the plain form:\n plain: %q\nstripped: %q", plainText, got)
	}
}

// stripANSI removes escape sequences. A CSI sequence is ESC '[' then parameter
// bytes then ONE final byte in @..~ — and the '[' is itself in that range, so a
// stripper must skip it before looking for the terminator. It must also accept
// terminators other than 'm': the live renderer emits cursor sequences
// (\x1b[1A, \x1b[2K) alongside colour ones.
func stripANSI(s string) string {
	var b strings.Builder
	var inEsc, seenBracket bool
	for _, r := range s {
		switch {
		case r == '\x1b':
			inEsc, seenBracket = true, false
		case inEsc && !seenBracket:
			// The introducer. Anything other than '[' is a short escape that
			// ends here.
			if r == '[' {
				seenBracket = true
			} else {
				inEsc = false
			}
		case inEsc:
			if r >= '@' && r <= '~' {
				inEsc = false
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// The verbosity ladder: what reaches the writer at each level.
func TestLevelFiltersEvents(t *testing.T) {
	events := []progress.Event{
		{Kind: progress.EvalStart, Phase: 0},
		{Kind: progress.EvalDone, Phase: 0, Duration: time.Second},
		{Kind: progress.PhaseStart, Phase: 0, Count: 1},
		{Kind: progress.NodeStart, ID: "alpha.a"},
		{Kind: progress.NodeDone, ID: "alpha.a", Duration: time.Second},
		{Kind: progress.ProviderSpawn, Name: "alpha"},
	}
	cases := []struct {
		level    Level
		contains []string
		absent   []string
	}{
		{LevelQuiet, nil, []string{"alpha.a", "Phase", "evaluating", "starting provider"}},
		{LevelInfo, []string{"Phase 1", "alpha.a"}, []string{"evaluating", "starting provider", "..."}},
		{LevelVerbose, []string{"Phase 1", "alpha.a", "evaluating configuration", "starting provider"}, nil},
		{LevelDebug, []string{"Phase 1", "alpha.a", "evaluating configuration"}, nil},
	}
	for _, c := range cases {
		t.Run(c.level.String(), func(t *testing.T) {
			got := render(c.level, false, events...)
			for _, want := range c.contains {
				if !strings.Contains(got, want) {
					t.Errorf("level %s should report %q; got:\n%s", c.level, want, got)
				}
			}
			for _, no := range c.absent {
				if strings.Contains(got, no) {
					t.Errorf("level %s should not report %q; got:\n%s", c.level, no, got)
				}
			}
		})
	}
}

// The default level reports what CHANGED, not what happened: an unchanged
// refresh is not named, a drifted one is. Naming forty unchanged reads buries
// the three that matter.
func TestDefaultLevelNamesDriftNotEveryRead(t *testing.T) {
	events := []progress.Event{
		{Kind: progress.NodeDone, ID: "alpha.unchanged1", Refresh: true},
		{Kind: progress.NodeDone, ID: "alpha.unchanged2", Refresh: true},
		{Kind: progress.NodeDone, ID: "alpha.drifted", Refresh: true, Drifted: true},
	}
	got := render(LevelInfo, false, events...)
	if strings.Contains(got, "unchanged1") || strings.Contains(got, "unchanged2") {
		t.Errorf("the default level must not name every refreshed resource:\n%s", got)
	}
	if !strings.Contains(got, "alpha.drifted") {
		t.Errorf("the default level must name the drifted ones:\n%s", got)
	}
}

// A higher level names every read, for a user who wants it.
func TestVerboseNamesEveryRead(t *testing.T) {
	events := []progress.Event{
		{Kind: progress.NodeDone, ID: "alpha.unchanged1", Refresh: true},
		{Kind: progress.NodeDone, ID: "alpha.drifted", Refresh: true, Drifted: true},
	}
	got := render(LevelVerbose, false, events...)
	for _, want := range []string{"alpha.unchanged1", "alpha.drifted"} {
		if !strings.Contains(got, want) {
			t.Errorf("verbose should name %q; got:\n%s", want, got)
		}
	}
}

// An event carrying an error renders nothing: the command reports the failure
// itself as a clean `error:` line, and a duplicate here would double-report it.
func TestFailureIsLeftToTheCommand(t *testing.T) {
	got := render(LevelInfo, false,
		progress.Event{Kind: progress.NodeDone, ID: "alpha.a", Err: errors.New("boom")},
		progress.Event{Kind: progress.EvalDone, Err: errors.New("boom")},
		progress.Event{Kind: progress.BuildDone, Name: "img", Err: errors.New("boom")},
	)
	if got != "" {
		t.Errorf("a failed event should not be rendered as narrative; got %q", got)
	}
}

// A note from another producer passes through as its own line.
func TestNotePassesThrough(t *testing.T) {
	got := render(LevelInfo, false, progress.Event{Kind: progress.Note, Message: "warning: deprecated"})
	if !strings.Contains(got, "warning: deprecated") {
		t.Errorf("a note should reach the output; got %q", got)
	}
}

// Suspend on a plain renderer just runs the function: there is nothing to tear
// down, and a subprocess may write freely.
func TestPlainSuspendRunsDirectly(t *testing.T) {
	var buf bytes.Buffer
	r := &plain{base: base{w: &buf, level: LevelInfo}}
	ran := false
	if err := r.Suspend(func() error { ran = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Error("Suspend did not run the function")
	}
}

// A writer handed to another producer reaches the same stream.
func TestPlainWriterReachesTheStream(t *testing.T) {
	var buf bytes.Buffer
	r := &plain{base: base{w: &buf, level: LevelInfo}}
	if _, err := r.Writer().Write([]byte("a provider note\n")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "a provider note") {
		t.Errorf("the writer did not reach the stream; got %q", buf.String())
	}
}
