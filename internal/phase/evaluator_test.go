// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package phase_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/phase"
)

// The Nix version is reported as context for the undocumented event stream the
// executor decodes. It must never fail a run: not knowing the version is worth
// reporting, not worth aborting for.
func TestNixVersion(t *testing.T) {
	got := phase.NixVersion(context.Background())
	if got == "" {
		t.Fatal(`NixVersion returned empty; it should report a version or "unknown"`)
	}
	if got == "unknown" {
		t.Skip("no nix on PATH — the unknown path is what ran here, and it did not fail")
	}
	if !strings.ContainsRune(got, '.') {
		t.Errorf("version %q does not look like a version", got)
	}
}
