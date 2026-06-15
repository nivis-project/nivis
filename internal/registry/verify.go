// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// parseShasums parses a SHA256SUMS body ("<hex>  <filename>" per line) into a
// filename -> hex-sum map.
func parseShasums(body string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(body, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			out[fields[1]] = strings.ToLower(fields[0])
		}
	}
	return out
}

// verify computes the SHA-256 of data and requires it to equal the checksum
// listed for filename in sums. A mismatch (or a missing entry) is an error;
// callers MUST NOT use the bytes if verify returns non-nil.
func verify(data []byte, filename string, sums map[string]string) error {
	want, ok := sums[filename]
	if !ok {
		return fmt.Errorf("registry: no checksum for %q in SHA256SUMS", filename)
	}
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != want {
		return fmt.Errorf("registry: checksum mismatch for %q: expected %s, got %s", filename, want, got)
	}
	return nil
}
