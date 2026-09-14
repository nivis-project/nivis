// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package state

import "reflect"

// Drifted reports whether a refreshed attribute map differs MEANINGFULLY from
// the stored one.
//
// It is deliberately not reflect.DeepEqual. A provider read returns the resource
// decoded at its FULL schema, so the refreshed map carries every attribute the
// schema declares — including ones the stored state never had, present as nil.
// Refreshing a completely unchanged resource therefore produces a map that is
// not DeepEqual to what was stored:
//
//	stored:    {"id": "alpha-0", "value": "alpha::0"}
//	refreshed: {"id": "alpha-0", "value": "alpha::0", "label": nil}
//
// A DeepEqual comparison calls that drift, which would mark every resource in a
// stack as drifted and make "3 of 40 drifted" a lie. So an attribute that is
// absent on one side is treated as equivalent to a nil on the other, and only a
// value that genuinely differs counts.
func Drifted(stored, refreshed map[string]interface{}) bool {
	for k, rv := range refreshed {
		sv, ok := stored[k]
		if ok {
			if !reflect.DeepEqual(sv, rv) {
				return true
			}
			continue
		}
		// Present only in the refreshed map: drift only if it carries a value.
		if rv != nil {
			return true
		}
	}
	for k, sv := range stored {
		if _, ok := refreshed[k]; !ok && sv != nil {
			// An attribute the provider no longer reports, which had a value.
			return true
		}
	}
	return false
}
