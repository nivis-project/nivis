// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFetchHcloudReal really resolves, downloads, verifies, and caches the
// Hetzner hcloud provider from the OpenTofu registry + GitHub releases. It is
// skipped unless TERRAE_NIVIS_NET_TESTS=1, since it makes real network calls.
func TestFetchHcloudReal(t *testing.T) {
	if os.Getenv("TERRAE_NIVIS_NET_TESTS") != "1" {
		t.Skip("network test; set TERRAE_NIVIS_NET_TESTS=1 to run")
	}
	c := New(t.TempDir())
	bin, err := c.Fetch(context.Background(), "hetznercloud/hcloud")
	if err != nil {
		t.Fatalf("fetch hcloud: %v", err)
	}
	if !strings.Contains(filepath.Base(bin), "terraform-provider-hcloud") {
		t.Errorf("unexpected binary: %s", bin)
	}
	fi, err := os.Stat(bin)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("verified binary missing/empty: %v", err)
	}
	if fi.Mode().Perm()&0o100 == 0 {
		t.Errorf("provider binary not executable: %v", fi.Mode())
	}

	// Second fetch must be a cache hit (we can't easily assert "no network", but
	// it must succeed and return the same path).
	bin2, err := c.Fetch(context.Background(), "hetznercloud/hcloud")
	if err != nil || bin2 != bin {
		t.Errorf("second fetch (cache) = %q, %v; want same path %q", bin2, err, bin)
	}
}
