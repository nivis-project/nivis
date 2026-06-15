// Copyright 2026 WeareTechnative B.V. and the nixform authors
// SPDX-License-Identifier: Apache-2.0

// Package fakeprovider provides the shared tfprotov6 plumbing for nixform's
// in-repo fake providers (DESIGN D6). Concrete providers (provider-alpha,
// provider-beta) declare a small set of Resources and serve via tf6server;
// everything protocol-level lives here and is tested once.
package fakeprovider

import (
	"os"
	"strconv"
	"sync/atomic"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Counter is a per-process, seedable counter used to make computed outputs
// deterministic for tests. It is seeded from NIXFORM_FAKE_COUNTER (default 0)
// and incremented once per applied resource. No clocks, no randomness.
type Counter struct{ n int64 }

// NewCounter reads the NIXFORM_FAKE_COUNTER env var (default 0) as the starting
// value. The first Next() returns the seed, the second seed+1, and so on.
func NewCounter() *Counter {
	start := int64(0)
	if s := os.Getenv("NIXFORM_FAKE_COUNTER"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			start = v
		}
	}
	// store start-1 so the first atomic.AddInt64 yields `start`.
	return &Counter{n: start - 1}
}

// Next returns the next counter value, incrementing atomically so concurrent
// applies remain deterministic per call order.
func (c *Counter) Next() int64 { return atomic.AddInt64(&c.n, 1) }

// objectType builds the tftypes.Object type for a resource from its attribute
// type map. The wire representation of a resource's config/state is always an
// object keyed by attribute name.
func objectType(attrs map[string]tftypes.Type) tftypes.Object {
	return tftypes.Object{AttributeTypes: attrs}
}

// decode unmarshals a protocol DynamicValue into a tftypes.Value of the given
// object type. A nil DynamicValue decodes to a null object (e.g. PriorState on
// create, or ProposedNewState on delete).
func decode(dv *tfprotov6.DynamicValue, typ tftypes.Object) (tftypes.Value, error) {
	if dv == nil {
		return tftypes.NewValue(typ, nil), nil
	}
	return dv.Unmarshal(typ)
}

// encode marshals a tftypes.Value back into a protocol DynamicValue.
func encode(typ tftypes.Object, v tftypes.Value) (*tfprotov6.DynamicValue, error) {
	dv, err := tfprotov6.NewDynamicValue(typ, v)
	if err != nil {
		return nil, err
	}
	return &dv, nil
}

// asObject reads a tftypes object value into its attribute map. A null or
// unknown object yields an empty map so callers can treat absent attrs as null.
func asObject(v tftypes.Value) (map[string]tftypes.Value, error) {
	if !v.IsKnown() || v.IsNull() {
		return map[string]tftypes.Value{}, nil
	}
	m := map[string]tftypes.Value{}
	if err := v.As(&m); err != nil {
		return nil, err
	}
	return m, nil
}

// optString reads an attribute as a *string: nil if the attr is absent, null,
// or unknown; otherwise the concrete string.
func optString(m map[string]tftypes.Value, key string) (*string, error) {
	v, ok := m[key]
	if !ok || v.IsNull() || !v.IsKnown() {
		return nil, nil
	}
	var s string
	if err := v.As(&s); err != nil {
		return nil, err
	}
	return &s, nil
}

// errDiag builds an error-severity diagnostic.
func errDiag(summary, detail string) *tfprotov6.Diagnostic {
	return &tfprotov6.Diagnostic{
		Severity: tfprotov6.DiagnosticSeverityError,
		Summary:  summary,
		Detail:   detail,
	}
}
