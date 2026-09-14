// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package ui

import (
	"bytes"
	"testing"
)

// clearEnv removes every variable the capability rules consult, so a test sees
// a known environment rather than the developer's.
// Note it UNSETS rather than setting to "": t.Setenv(k, "") leaves the variable
// PRESENT, and NO_COLOR's contract is that presence alone disables colour.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"NO_COLOR", "CLICOLOR_FORCE", "CI", "GITHUB_ACTIONS", "GITLAB_CI",
		"BUILDKITE", "TEAMCITY_VERSION",
	} {
		unsetEnvForTest(t, k)
	}
	t.Setenv("TERM", "xterm-256color")
}

func TestColorModeOverridesEnvironment(t *testing.T) {
	var buf bytes.Buffer // not a terminal

	clearEnv(t)
	if ColorEnabled(&buf, ColorAlways) != true {
		t.Error("--color=always must colourise even a pipe (a CI system that renders ANSI asks for this)")
	}
	if ColorEnabled(&buf, ColorNever) != false {
		t.Error("--color=never must never colourise")
	}
	if ColorEnabled(&buf, ColorAuto) != false {
		t.Error("auto must not colourise a pipe")
	}

	// NO_COLOR disables under auto...
	t.Setenv("NO_COLOR", "1")
	if ColorEnabled(&buf, ColorAuto) != false {
		t.Error("NO_COLOR must disable colour under auto")
	}
	// ...but an explicit --color=always wins over it.
	if ColorEnabled(&buf, ColorAlways) != true {
		t.Error("an explicit --color=always must win over NO_COLOR")
	}
}

func TestClicolorForceEnablesUnderAuto(t *testing.T) {
	clearEnv(t)
	var buf bytes.Buffer
	t.Setenv("CLICOLOR_FORCE", "1")
	if ColorEnabled(&buf, ColorAuto) != true {
		t.Error("CLICOLOR_FORCE should enable colour under auto, as NO_COLOR's counterpart")
	}
	// NO_COLOR still wins.
	t.Setenv("NO_COLOR", "1")
	if ColorEnabled(&buf, ColorAuto) != false {
		t.Error("NO_COLOR should win over CLICOLOR_FORCE")
	}
}

// A `dumb` terminal is neither colour- nor cursor-capable.
func TestDumbTerminalIsNotCapable(t *testing.T) {
	clearEnv(t)
	t.Setenv("TERM", "dumb")
	var buf bytes.Buffer
	if ColorEnabled(&buf, ColorAuto) {
		t.Error("TERM=dumb must not be treated as colour-capable under auto")
	}
	if CursorEnabled(&buf, LevelInfo) {
		t.Error("TERM=dumb must not be driven with cursor control")
	}
}

// The decisive separation: cursor capability is NOT colour capability. A
// terminal can render colour and still mishandle cursor movement, and
// conflating them corrupts output on exactly the terminals least able to cope.
//
// The user-visible consequence: --color=never leaves the live region on (a user
// turning off colour did not ask to turn off progress), and forcing colour into
// a pipe does not turn the region on (a pipe has no cursor).
func TestCursorCapabilityIsSeparateFromColour(t *testing.T) {
	clearEnv(t)
	var buf bytes.Buffer // a pipe

	if CursorEnabled(&buf, LevelInfo) {
		t.Error("a pipe has no cursor, whatever the colour setting")
	}
	// Forcing colour must not imply a cursor.
	if ColorEnabled(&buf, ColorAlways) != true || CursorEnabled(&buf, LevelInfo) != false {
		t.Error("--color=always must not enable the live region on a pipe")
	}
	// And the reverse: disabling colour is not a statement about the cursor, so
	// the two rules must not consult each other. NO_COLOR kills colour...
	t.Setenv("NO_COLOR", "1")
	if ColorEnabled(&buf, ColorAuto) != false {
		t.Error("NO_COLOR should disable colour")
	}
	// ...and CursorEnabled must not even look at it: its answer here is driven
	// by the pipe, not by NO_COLOR, which is what the next case pins.
}

// The live region is confined to the default verbosity, which is the other half
// of the mutual-exclusion rule: at verbose and debug a subprocess owns the
// cursor instead.
func TestLiveRegionOnlyAtDefaultVerbosity(t *testing.T) {
	clearEnv(t)
	var buf bytes.Buffer
	for _, l := range []Level{LevelQuiet, LevelVerbose, LevelDebug} {
		if CursorEnabled(&buf, l) {
			t.Errorf("the live region must be off at %s", l)
		}
	}
}

// A CI environment gets the plain renderer: a rewritten-in-place region
// produces a log full of control characters rather than a transcript.
func TestCIGetsPlainRendering(t *testing.T) {
	clearEnv(t)
	t.Setenv("CI", "true")
	var buf bytes.Buffer
	if CursorEnabled(&buf, LevelInfo) {
		t.Error("CI must not get a live region")
	}
}

// The mutual-exclusion rule, as a table: raw subprocess output and the live
// region are never both active.
func TestSubprocessOutputAndLiveRegionAreExclusive(t *testing.T) {
	clearEnv(t)
	var buf bytes.Buffer
	for _, l := range []Level{LevelQuiet, LevelInfo, LevelVerbose, LevelDebug} {
		passthrough := l.PassThroughSubprocessOutput()
		live := CursorEnabled(&buf, l)
		if passthrough && live {
			t.Errorf("at %s both raw subprocess output and the live region are active; "+
				"they fight over the cursor", l)
		}
	}
}

func TestParseLevel(t *testing.T) {
	for _, name := range LevelNames() {
		if _, ok := ParseLevel(name); !ok {
			t.Errorf("%q should be an accepted level", name)
		}
	}
	if l, ok := ParseLevel(""); !ok || l != DefaultLevel {
		t.Errorf("an empty level should mean the default, got %v ok=%v", l, ok)
	}
	if _, ok := ParseLevel("chatty"); ok {
		t.Error("an unknown level must be refused, not silently defaulted")
	}
}

func TestParseColorMode(t *testing.T) {
	for _, name := range ColorModeNames() {
		if _, ok := ParseColorMode(name); !ok {
			t.Errorf("%q should be an accepted colour mode", name)
		}
	}
	if _, ok := ParseColorMode("maybe"); ok {
		t.Error("an unknown colour mode must be refused")
	}
}

func TestPassThroughTable(t *testing.T) {
	want := map[Level]bool{
		LevelQuiet: false, LevelInfo: false, LevelVerbose: true, LevelDebug: true,
	}
	for l, w := range want {
		if got := l.PassThroughSubprocessOutput(); got != w {
			t.Errorf("%s: passthrough = %v, want %v", l, got, w)
		}
	}
}
