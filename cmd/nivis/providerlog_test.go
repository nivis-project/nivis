// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

// The CLI surface of provider notes: the --provider-log-level flag, where notes
// are written, and how they are styled.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nivis-project/nivis/internal/providerlog"
)

// withProviderLogLevel sets the flag's package-global for the test's duration.
func withProviderLogLevel(t *testing.T, value string) {
	t.Helper()
	old := providerLogLevel
	providerLogLevel = value
	t.Cleanup(func() { providerLogLevel = old })
}

// The default is the level that keeps provider warnings visible as notes: the
// design's decision 1 (fix noise by rendering, not by hiding).
func TestProviderLogLevelDefault(t *testing.T) {
	withProviderLogLevel(t, providerlog.DefaultLevel.String())
	_, sink, err := newManager(&bytes.Buffer{})
	if err != nil {
		t.Fatalf("newManager: %v", err)
	}
	if sink == nil {
		t.Fatal("a sink should always be attached")
	}
	if providerlog.DefaultLevel != providerlog.LevelWarn {
		t.Errorf("the default provider log level is %v, want warn", providerlog.DefaultLevel)
	}
}

// Every accepted level is accepted, in any case.
func TestProviderLogLevelAccepted(t *testing.T) {
	for _, name := range providerlog.LevelNames() {
		for _, spelling := range []string{name, strings.ToUpper(name)} {
			withProviderLogLevel(t, spelling)
			if _, _, err := newManager(&bytes.Buffer{}); err != nil {
				t.Errorf("--provider-log-level=%s rejected: %v", spelling, err)
			}
		}
	}
}

// An unknown value is refused before anything is spawned, naming the value and
// the accepted set — never a silent fallback to the default.
func TestProviderLogLevelUnknownIsRefused(t *testing.T) {
	withProviderLogLevel(t, "verbose")
	_, _, err := newManager(&bytes.Buffer{})
	if err == nil {
		t.Fatal("an unknown --provider-log-level must be refused")
	}
	for _, want := range append([]string{"verbose"}, providerlog.LevelNames()...) {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error should name %q: %v", want, err)
		}
	}
}

// The flag is global, so it reaches every provider-spawning command.
func TestProviderLogLevelIsAGlobalFlag(t *testing.T) {
	// Each provider-spawning command must rely on the GLOBAL flag rather than
	// defining its own, so there is a single knob.
	builders := map[string]func() *cobra.Command{
		"plan":    planCmd,
		"apply":   applyCmd,
		"destroy": destroyCmd,
		"refresh": refreshCmd,
		"output":  outputCmd,
		"gen":     genCmd,
	}
	for name, build := range builders {
		t.Run(name, func(t *testing.T) {
			cmd := build()
			if cmd == nil {
				t.Fatalf("%s command is nil", name)
			}
			if f := cmd.Flags().Lookup("provider-log-level"); f != nil {
				t.Errorf("%s defines its own provider-log-level flag; it should use the global one", name)
			}
		})
	}
}

// Notes are written to the writer newManager is given (the command's stderr),
// so the change list on stdout stays unmixed.
func TestNotesGoToTheGivenWriter(t *testing.T) {
	withProviderLogLevel(t, "warn")
	var notes bytes.Buffer
	_, sink, err := newManager(&notes)
	if err != nil {
		t.Fatalf("newManager: %v", err)
	}
	sink.Note("aws", []byte(`{"@level":"warn","@message":"m","tf_resource_type":"t"}`))
	if !strings.Contains(notes.String(), "provider note t: m") {
		t.Errorf("the note should have been written to the given writer, got:\n%s", notes.String())
	}
}

// A non-TTY writer gets no ANSI codes, so piped output and test assertions are
// stable (the same rule the rest of the CLI's output follows).
func TestNotesHonourNoColorForPipedOutput(t *testing.T) {
	withProviderLogLevel(t, "warn")
	t.Setenv("NO_COLOR", "1")
	var notes bytes.Buffer
	_, sink, err := newManager(&notes)
	if err != nil {
		t.Fatalf("newManager: %v", err)
	}
	sink.Note("aws", []byte(`{"@level":"warn","@message":"m","tf_resource_type":"t"}`))
	if strings.Contains(notes.String(), "\x1b[") {
		t.Errorf("piped output must carry no ANSI codes: %q", notes.String())
	}
}
