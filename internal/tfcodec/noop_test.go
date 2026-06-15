// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

package tfcodec

import "testing"

// KnownAttrsMatchPrior is the no-op signal: known planned attrs must equal prior;
// attrs the provider re-marks unknown-after-apply (computed: arn, etag, …) are
// ignored, since for an unchanged resource the provider keeps their prior values.
func TestKnownAttrsMatchPrior(t *testing.T) {
	prior := map[string]interface{}{
		"key":     "hello.txt",
		"content": "from nix",
		"arn":     "arn:aws:s3:::b/hello.txt", // a computed attr
		"etag":    "abc123",                   // a computed attr
	}

	// Re-plan of an UNCHANGED resource: the provider re-marks arn/etag unknown
	// and may omit them from the planned map; the known attrs match prior.
	planned := map[string]interface{}{
		"key":     "hello.txt",
		"content": "from nix",
	}
	if !KnownAttrsMatchPrior(planned, prior, []string{"arn", "etag"}) {
		t.Error("unchanged resource (computed attrs unknown) should be a no-op")
	}

	// A real change to a known attr is NOT a no-op.
	changed := map[string]interface{}{
		"key":     "hello.txt",
		"content": "DIFFERENT",
	}
	if KnownAttrsMatchPrior(changed, prior, []string{"arn", "etag"}) {
		t.Error("a changed known attr must not be a no-op")
	}

	// A known attr present in the plan but unknown-listed is skipped (no change).
	if !KnownAttrsMatchPrior(
		map[string]interface{}{"key": "hello.txt", "content": "from nix", "etag": "WILL-RECOMPUTE"},
		prior,
		[]string{"etag"},
	) {
		t.Error("an unknown-listed attr must be ignored even if present/differing")
	}
}
