// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package ui renders what the engines report. It is the only thing that writes
// to the user's terminal.
//
// The split it completes: the engines emit facts (internal/progress), and every
// decision about wording, colour, verbosity and terminal capability lives here.
// A renderer owns the stream it writes to — exclusively — because a live region
// rewrites lines in place, and any other writer emitting a line mid-redraw
// corrupts the display with no way to recover.
package ui

import (
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// Brand ANSI colours (24-bit truecolor) from docs/BRAND.md:
//
//	ember #F2632E (the one warm accent), ice blue #AECFE6, silver #C3D2DE.
const (
	Reset = "\x1b[0m"
	Ember = "\x1b[38;2;242;99;46m"   // Volcanic Ember
	Ice   = "\x1b[38;2;174;207;230m" // Ice Blue
	Dim   = "\x1b[38;2;120;140;160m" // dim grey-blue
	Snow  = "\x1b[1;38;2;245;250;253m"

	// Change-type colours. Green/yellow/red carry the conventional plan-diff
	// meaning.
	Green  = "\x1b[38;2;90;200;120m" // create
	Yellow = "\x1b[38;2;220;190;90m" // update
	Red    = "\x1b[38;2;220;100;90m" // destroy / replace (destroy half)
)

// ColorMode is the --color setting.
type ColorMode int

const (
	ColorAuto ColorMode = iota
	ColorAlways
	ColorNever
)

// ParseColorMode maps a --color value to a mode, refusing anything else rather
// than falling back silently.
func ParseColorMode(s string) (ColorMode, bool) {
	switch s {
	case "auto", "":
		return ColorAuto, true
	case "always":
		return ColorAlways, true
	case "never":
		return ColorNever, true
	}
	return ColorAuto, false
}

// ColorModeNames lists the accepted --color values, for help text and errors.
func ColorModeNames() []string { return []string{"auto", "always", "never"} }

// isDumb reports whether TERM names a terminal that cannot be driven. Such a
// terminal is neither colour- nor cursor-capable.
func isDumb() bool {
	t := os.Getenv("TERM")
	return t == "dumb" || t == ""
}

// isTTY reports whether w is a terminal.
func isTTY(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

// ColorEnabled reports whether ANSI colour should be used for w under mode.
//
//   - never  : off, always.
//   - always : on, always — including into a pipe, which is what a CI system
//     that renders ANSI asks for.
//   - auto   : on for a colour-capable terminal with colour not disabled by the
//     environment (https://no-color.org, and CLICOLOR_FORCE as its counterpart).
//
// CLICOLOR_FORCE behaves as `--color=always`; NO_COLOR wins over it, and an
// explicit --color wins over both.
func ColorEnabled(w io.Writer, mode ColorMode) bool {
	switch mode {
	case ColorNever:
		return false
	case ColorAlways:
		return true
	}
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}
	if v, ok := os.LookupEnv("CLICOLOR_FORCE"); ok && v != "0" {
		return true
	}
	if isDumb() {
		return false
	}
	return isTTY(w)
}

// CursorEnabled reports whether w can be driven with cursor control, which is
// what a live, rewritten-in-place region requires.
//
// This is deliberately a SEPARATE question from colour. A terminal can render
// colour correctly and still mishandle cursor movement, and conflating the two
// corrupts output on exactly the terminals least able to cope. So neither
// --color=never nor NO_COLOR disables the live region, and forcing colour into
// a pipe does not enable it: a pipe has no cursor.
//
// The live region is also confined to the default verbosity. At `quiet` there
// is nothing to show, and at `verbose`/`debug` a subprocess's own output is
// passed through and owns the cursor instead (see the Level table).
func CursorEnabled(w io.Writer, level Level) bool {
	if level != LevelInfo {
		return false
	}
	if isDumb() {
		return false
	}
	if isCI() {
		return false
	}
	return isTTY(w)
}

// isCI reports whether we appear to be running in a continuous-integration
// environment, where a rewritten-in-place region produces a log full of control
// characters rather than a readable transcript.
func isCI() bool {
	for _, k := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "BUILDKITE", "TEAMCITY_VERSION"} {
		if v, ok := os.LookupEnv(k); ok && v != "" && v != "0" && !strings.EqualFold(v, "false") {
			return true
		}
	}
	return false
}
