// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

// The CLI-level wiring: the flags, their precedence, and the channel split.

import (
	"os"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/ui"
)

func withLogLevel(t *testing.T, v string) {
	t.Helper()
	old := logLevel
	logLevel = v
	t.Cleanup(func() { logLevel = old })
}

func withColorMode(t *testing.T, v string) {
	t.Helper()
	old := colorMode
	colorMode = v
	t.Cleanup(func() { colorMode = old })
}

// unsetNivisLog removes NIVIS_LOG for the test. t.Setenv cannot express
// "absent", and an empty value must be treated as not given.
func unsetNivisLog(t *testing.T) {
	t.Helper()
	old, had := os.LookupEnv("NIVIS_LOG")
	_ = os.Unsetenv("NIVIS_LOG")
	t.Cleanup(func() {
		if had {
			_ = os.Setenv("NIVIS_LOG", old)
		} else {
			_ = os.Unsetenv("NIVIS_LOG")
		}
	})
}

func TestLogLevelDefaultsToInfo(t *testing.T) {
	unsetNivisLog(t)
	withLogLevel(t, "")
	got, err := resolveLogLevel()
	if err != nil {
		t.Fatal(err)
	}
	if got != ui.DefaultLevel {
		t.Errorf("default level = %v, want the package default %v", got, ui.DefaultLevel)
	}
	if ui.DefaultLevel != ui.LevelInfo {
		t.Errorf("the default level is %v, want info", ui.DefaultLevel)
	}
}

func TestLogLevelFromEnvironment(t *testing.T) {
	withLogLevel(t, "")
	t.Setenv("NIVIS_LOG", "verbose")
	got, err := resolveLogLevel()
	if err != nil {
		t.Fatal(err)
	}
	if got != ui.LevelVerbose {
		t.Errorf("level = %v, want verbose from NIVIS_LOG", got)
	}
}

// An explicit flag wins over the environment, so a CI system can set the
// variable once without preventing a one-off override.
func TestLogLevelFlagBeatsEnvironment(t *testing.T) {
	t.Setenv("NIVIS_LOG", "verbose")
	withLogLevel(t, "quiet")
	got, err := resolveLogLevel()
	if err != nil {
		t.Fatal(err)
	}
	if got != ui.LevelQuiet {
		t.Errorf("level = %v, want quiet (the flag must beat NIVIS_LOG)", got)
	}
}

// An unrecognised value is refused, naming what is accepted — never a silent
// fallback that mistakes a typo for a preference.
func TestUnknownLogLevelIsRefused(t *testing.T) {
	unsetNivisLog(t)
	withLogLevel(t, "chatty")
	_, err := resolveLogLevel()
	if err == nil {
		t.Fatal("an unknown --log-level must be refused")
	}
	for _, want := range []string{"chatty", "quiet", "info", "verbose", "debug"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error should name %q; got %v", want, err)
		}
	}
}

func TestUnknownNivisLogIsRefused(t *testing.T) {
	withLogLevel(t, "")
	t.Setenv("NIVIS_LOG", "loud")
	if _, err := resolveLogLevel(); err == nil {
		t.Fatal("an unknown NIVIS_LOG must be refused")
	}
}

func TestUnknownColorModeIsRefused(t *testing.T) {
	unsetNivisLog(t)
	withLogLevel(t, "")
	withColorMode(t, "maybe")
	_, err := newRenderer(os.Stderr)
	if err == nil {
		t.Fatal("an unknown --color must be refused")
	}
	for _, want := range []string{"maybe", "auto", "always", "never"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error should name %q; got %v", want, err)
		}
	}
}

// The two log-level flags govern different things and must stay independent:
// --log-level silences NIVIS, --provider-log-level silences the PROVIDERS.
// Neither should be an alias of the other.
func TestLogLevelAndProviderLogLevelAreIndependent(t *testing.T) {
	unsetNivisLog(t)
	withLogLevel(t, "quiet")
	withProviderLogLevel(t, "warn")

	level, err := resolveLogLevel()
	if err != nil {
		t.Fatal(err)
	}
	if level != ui.LevelQuiet {
		t.Fatalf("nivis level = %v, want quiet", level)
	}
	// The provider sink is still built at its own level: silencing Nivis's
	// narrative must not silence a provider's deprecation warning.
	_, sink, err := newManager(ui.Discard())
	if err != nil {
		t.Fatalf("newManager: %v", err)
	}
	if sink == nil {
		t.Error("a provider-log sink should still be attached at --log-level=quiet")
	}
}

// The mutual-exclusion rule as the CLI applies it: the terminal policy passes
// subprocess output through only at the levels where the live region is off.
func TestTerminalPolicyMatchesTheLevel(t *testing.T) {
	for _, c := range []struct {
		level       ui.Level
		passthrough bool
	}{
		{ui.LevelQuiet, false},
		{ui.LevelInfo, false},
		{ui.LevelVerbose, true},
		{ui.LevelDebug, true},
	} {
		r := ui.Discard()
		term := terminalFor(r, c.level)
		if (term.Stderr != nil) != c.passthrough {
			t.Errorf("level %s: subprocess passthrough = %v, want %v",
				c.level, term.Stderr != nil, c.passthrough)
		}
		if term.Suspend == nil {
			t.Errorf("level %s: the terminal policy must always be able to suspend, "+
				"so a subprocess can never collide with a live region", c.level)
		}
	}
}

// The renderer is the SOLE owner of stderr. The provider-log sink must be given
// the renderer's writer, not the raw stream: a live region rewrites lines in
// place, and an independent writer emitting a note mid-redraw corrupts it with
// no way to recover.
func TestProviderNotesGoThroughTheRenderer(t *testing.T) {
	unsetNivisLog(t)
	withLogLevel(t, "")
	withColorMode(t, "never")
	withProviderLogLevel(t, "warn")

	var stream strings.Builder
	r := ui.New(&stream, ui.LevelInfo, ui.ColorNever)
	_, sink, err := newManager(r)
	if err != nil {
		t.Fatalf("newManager: %v", err)
	}

	// A note rendered by the sink must reach the renderer's stream.
	sink.Note("alpha", []byte(`{"@level":"warn","@message":"deprecated attribute"}`))
	if !strings.Contains(stream.String(), "deprecated attribute") {
		t.Errorf("the note did not reach the renderer's stream; got %q", stream.String())
	}
}
