// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package nixlog_test

// The fresh-capture check, driven by tests/check-nix-logformat.sh.
//
// The committed fixtures guard one direction: a DECODER change that breaks an
// older nix. They cannot guard the other — a NEWER nix changing the stream —
// because they are recordings. This test decodes a capture taken moments ago
// from whatever nix is installed, which in CI may be newer than the flake's
// pin.
//
// It is skipped unless the script hands it a capture, so `go test ./...` stays
// hermetic and needs no nix.

import (
	"os"
	"testing"
)

func TestFreshCapture(t *testing.T) {
	path := os.Getenv("NIVIS_FRESH_CAPTURE")
	if path == "" {
		t.Skip("no NIVIS_FRESH_CAPTURE: run tests/check-nix-logformat.sh")
	}

	lines := readLines(t, path)
	ver := fixtureVersion(lines)
	f := decodeAll(lines)
	t.Logf("nix %s: builds=%d count=%d/%d logLines=%d unreadable=%d",
		ver, len(f.builds), f.maxDone, f.maxExp, f.logLines, f.unread)

	// The same facts the committed fixtures must yield. A failure here means
	// THIS nix produces something the decoder cannot read — a contract finding
	// about a version, not a stale recording.
	if len(f.builds) != 3 {
		t.Errorf("nix %s named %d build(s) %v, want 3 — this version's stream has moved",
			ver, len(f.builds), f.builds)
	}
	if f.maxDone < 3 || f.maxExp < 3 {
		t.Errorf("nix %s: count reached %d/%d, want at least 3/3", ver, f.maxDone, f.maxExp)
	}
	if f.logLines == 0 {
		t.Errorf("nix %s: the build's own output never arrived", ver)
	}
	if f.unread > 0 {
		// Not fatal: degradation covers unrecognised events by design. But a
		// rise here is the earliest signal that a new nix has changed shape.
		t.Logf("note: %d structured line(s) were unreadable on nix %s — "+
			"tolerated by design, but worth a look if this is new", f.unread, ver)
	}
}
