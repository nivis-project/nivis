// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// splash to a non-TTY buffer must be plain (no ANSI) and contain the brand text.
func TestSplashPlainToBuffer(t *testing.T) {
	var buf bytes.Buffer
	splash(&buf)
	out := buf.String()

	if !strings.Contains(out, "TERRAE NIVIS") {
		t.Errorf("splash missing wordmark:\n%s", out)
	}
	if !strings.Contains(out, "infrastructure as nix code") {
		t.Errorf("splash missing tagline:\n%s", out)
	}
	// A bytes.Buffer is not a TTY, so output must carry no ANSI escape codes.
	if strings.Contains(out, "\x1b[") {
		t.Errorf("splash to a non-TTY must not contain ANSI escapes:\n%q", out)
	}
}

func TestColorEnabledNonTTY(t *testing.T) {
	var buf bytes.Buffer
	if colorEnabled(&buf) {
		t.Error("colorEnabled must be false for a non-*os.File writer")
	}
}

func TestNoColorEnvDisables(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	// Even stdout (a *os.File) must be treated as no-color when NO_COLOR is set.
	if colorEnabled(os.Stdout) {
		t.Error("NO_COLOR must disable colour even on a TTY")
	}
}
