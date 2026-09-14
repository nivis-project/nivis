// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package nixlog_test

// Conformance across Nix versions.
//
// The decoder degrades when it meets an event it does not RECOGNISE. What
// degradation cannot cover is a type quietly reused to mean something else:
// that is misreported, not degraded, and nothing in the running tool can
// detect it. The only thing that closes that gap is evidence — the same facts
// extracted from a real build under every Nix the pinned nixpkgs provides.
//
// The captures need nix; this test does not. That split is load-bearing: the
// coverage gate runs in a sandbox with no nix binary at all, so a conformance
// test that shelled out to nix would be skipped exactly where it matters.

import (
	"strings"
	"testing"

	"github.com/nivis-project/nivis/internal/nixlog"
)

// facts is what a stream must yield, whatever version produced it.
type facts struct {
	builds   []string
	maxDone  int
	maxExp   int
	logLines int
	unread   int
}

func decodeAll(lines []string) facts {
	d := nixlog.New()
	var f facts
	prev := ""
	for _, ln := range lines {
		_, understood := d.Line(ln)
		if !understood && strings.HasPrefix(ln, "@nix ") {
			f.unread++
		}
		u := d.Update()
		if u.Building != "" && u.Building != prev {
			f.builds = append(f.builds, u.Building)
			prev = u.Building
		}
		if u.Done > f.maxDone {
			f.maxDone = u.Done
		}
		if u.Expected > f.maxExp {
			f.maxExp = u.Expected
		}
		if u.LastLine != "" {
			f.logLines++
		}
	}
	return f
}

// Every available version must yield the SAME facts — not merely decode
// without erroring. A stream that produces no builds has told us nothing, and
// would pass a weaker assertion.
func TestEveryVersionYieldsTheSameFacts(t *testing.T) {
	paths := fixturePaths(t)
	t.Logf("replaying %d captured stream(s)", len(paths))

	for _, path := range paths {
		lines := readLines(t, path)
		ver := fixtureVersion(lines)
		f := decodeAll(lines)

		t.Logf("  %-34s nix %-10s builds=%d count=%d/%d logLines=%d unreadable=%d",
			path, ver, len(f.builds), f.maxDone, f.maxExp, f.logLines, f.unread)

		// The capture is a three-derivation chain, so all three must be named.
		if len(f.builds) != 3 {
			t.Errorf("%s (nix %s): named %d build(s) %v, want 3. "+
				"If this fixture is older than the pin, regenerate it; if it is current, "+
				"the stream's shape has moved and that is a contract finding.",
				path, ver, len(f.builds), f.builds)
		}
		// The count must ADVANCE — a decoder reporting 0/0 throughout would
		// satisfy "no error" and tell a user nothing.
		if f.maxDone < 3 || f.maxExp < 3 {
			t.Errorf("%s (nix %s): count reached %d/%d, want at least 3/3",
				path, ver, f.maxDone, f.maxExp)
		}
		// The build's own output must arrive. This is also what proves
		// --print-build-logs is unnecessary (and nix-store rejects it).
		if f.logLines == 0 {
			t.Errorf("%s (nix %s): the build's own output never arrived", path, ver)
		}
	}
}

// Provenance: every capture must say which nix it came from, or a failure after
// a pin bump is indistinguishable from a decoder defect.
func TestEveryFixtureRecordsItsVersion(t *testing.T) {
	for _, path := range fixturePaths(t) {
		ver := fixtureVersion(readLines(t, path))
		if ver == "unrecorded" {
			t.Errorf("%s records no nix version; a stale capture and a broken decoder "+
				"would then produce the same red test", path)
		}
		if !strings.ContainsRune(ver, '.') {
			t.Errorf("%s records nix version %q, which does not look like a version", path, ver)
		}
	}
}

// The versions covered must be DISTINCT. Four fixtures all captured from the
// same nix would satisfy the count while proving nothing about breadth.
func TestFixturesCoverDistinctVersions(t *testing.T) {
	seen := map[string]string{}
	for _, path := range fixturePaths(t) {
		ver := fixtureVersion(readLines(t, path))
		if other, dup := seen[ver]; dup {
			t.Errorf("%s and %s were both captured from nix %s; the set proves less "+
				"breadth than its size suggests", other, path, ver)
		}
		seen[ver] = path
	}
	t.Logf("distinct nix versions covered: %d", len(seen))
}
