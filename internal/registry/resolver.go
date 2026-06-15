// Copyright 2026 WeareTechnative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"os"
)

// ResolveProvider implements the plugin manager's Resolver: an existing
// filesystem path is returned as-is; otherwise the source is treated as a
// registry address and fetched (resolve + download + verify + cache).
func (c *Client) ResolveProvider(ctx context.Context, source string) (string, error) {
	if _, err := os.Stat(source); err == nil {
		return source, nil // a real file: use it directly
	}
	if !LooksLikeAddress(source) {
		// Not a file and not address-shaped: surface the original (spawn will
		// fail with a clear "no such file" rather than a confusing registry error).
		return source, nil
	}
	return c.Fetch(ctx, source)
}
