// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"encoding/json"
	"fmt"
)

// The canonical state-document format and its (de)serialization, shared by every
// Store backend (local file, S3, ...) so whole-state read/replace and the on-disk
// shape are identical across backends.

// parseDocument decodes state-document bytes leniently: empty input is a fresh
// empty document, and a nil resources map is normalized to an empty one. Used for
// loading current state (where an absent/empty object is a fresh stack).
func parseDocument(data []byte) (document, error) {
	doc := document{Resources: map[string]ResourceState{}}
	if len(data) == 0 {
		return doc, nil
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, fmt.Errorf("invalid state document: %w", err)
	}
	if doc.Resources == nil {
		doc.Resources = map[string]ResourceState{}
	}
	return doc, nil
}

// parseDocumentStrict decodes bytes for Restore: it validates the document
// invariant that each entry's map key matches its resource id, so a malformed
// replacement is rejected before any backend writes it.
func parseDocumentStrict(data []byte) (document, error) {
	var doc document
	if err := json.Unmarshal(data, &doc); err != nil {
		return doc, fmt.Errorf("state: restore: input is not a valid state document: %w", err)
	}
	if doc.Resources == nil {
		doc.Resources = map[string]ResourceState{}
	}
	for id, rs := range doc.Resources {
		if rs.ID != id {
			return doc, fmt.Errorf("state: restore: resource key %q does not match its id %q", id, rs.ID)
		}
	}
	return doc, nil
}

// marshalDocument renders a document as canonical (indented) JSON bytes.
func marshalDocument(doc document) ([]byte, error) {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("state: marshal: %w", err)
	}
	return data, nil
}
