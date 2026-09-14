// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package ui

import "strings"

// Level is how much of Nivis's OWN narrative is reported. It is distinct from
// --provider-log-level, which governs the output of spawned providers.
type Level int

const (
	// LevelQuiet reports the result and errors only.
	LevelQuiet Level = iota
	// LevelInfo is the default: each unit of work as it completes, phase
	// boundaries, and a live region where the terminal allows.
	LevelInfo
	// LevelVerbose adds work as it STARTS, timings for evaluation and provider
	// startup, every refresh named, and subprocess output passed through raw.
	LevelVerbose
	// LevelDebug adds detail for diagnosing Nivis itself.
	LevelDebug
)

// DefaultLevel is the level used when neither the flag nor the environment
// selects one.
const DefaultLevel = LevelInfo

var levelNames = map[Level]string{
	LevelQuiet:   "quiet",
	LevelInfo:    "info",
	LevelVerbose: "verbose",
	LevelDebug:   "debug",
}

func (l Level) String() string {
	if s, ok := levelNames[l]; ok {
		return s
	}
	return "info"
}

// LevelNames lists the accepted values in increasing verbosity, for help text
// and error messages.
func LevelNames() []string { return []string{"quiet", "info", "verbose", "debug"} }

// ParseLevel maps a --log-level value to a Level. An unrecognised value is
// refused rather than silently falling back, so a typo is not mistaken for a
// preference.
func ParseLevel(s string) (Level, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "quiet":
		return LevelQuiet, true
	case "info", "":
		return LevelInfo, true
	case "verbose":
		return LevelVerbose, true
	case "debug":
		return LevelDebug, true
	}
	return DefaultLevel, false
}

// PassThroughSubprocessOutput reports whether a subprocess Nivis runs (a `nix
// eval`, a `nix-store --realise`) should have its own output reach the
// terminal at this level.
//
// This is one half of a rule the other half of which is CursorEnabled: raw
// subprocess output and the live region are MUTUALLY EXCLUSIVE, because Nix's
// progress display and Nivis's region both drive the cursor. Exactly one is
// active at a time:
//
//	level     nix output            live region
//	quiet     suppressed            off
//	info      suppressed            on
//	verbose   passed through raw    off
//	debug     passed through raw    off
func (l Level) PassThroughSubprocessOutput() bool { return l >= LevelVerbose }
