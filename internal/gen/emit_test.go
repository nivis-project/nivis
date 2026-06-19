// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package gen

import (
	"regexp"
	"strings"
	"testing"
)

// A provider attribute named like a reserved constructor parameter (name,
// overrides, nivis) must not duplicate or shadow that formal. It is taken under a
// cfg_<name> alias and emitted into config under its real key. Regression for
// nixform2-56tm (1379 real constructors emitted invalid Nix from this collision).
func TestEmitReservedNameCollision(t *testing.T) {
	r := Resource{
		Type: "thing",
		Attrs: []Attr{
			{Name: "id", Computed: true, Type: NixType{Kind: KindString}},
			{Name: "name", Required: true, Type: NixType{Kind: KindString}},
			{Name: "overrides", Optional: true, Type: NixType{Kind: KindString}},
			{Name: "label", Optional: true, Type: NixType{Kind: KindString}},
		},
	}
	out := Emit("p", r)

	// The reserved `name` formal (the instance name) appears exactly once in the
	// lambda head, never as `name ? null` (which would be the duplicate).
	head := lambdaHead(t, out)
	if n := countFormal(head, "name"); n != 1 {
		t.Errorf("`name` appears %d times as a formal, want exactly 1 (the instance name)\nhead: %s", n, head)
	}
	// `overrides` appears once: as the reserved `overrides ? {}` merge seam, not
	// duplicated by the provider attribute.
	if n := countFormal(head, "overrides"); n != 1 {
		t.Errorf("`overrides` appears %d times as a formal, want exactly 1 (the merge seam)\nhead: %s", n, head)
	}

	// The colliding attributes are taken under aliases.
	if !strings.Contains(head, "cfg_name ? null") {
		t.Errorf("provider `name` attr not aliased to cfg_name in the head:\n%s", head)
	}
	if !strings.Contains(head, "cfg_overrides ? null") {
		t.Errorf("provider `overrides` attr not aliased to cfg_overrides in the head:\n%s", head)
	}

	// config gets the provider attributes under their REAL keys, read from aliases.
	if !strings.Contains(out, "name = _cfg_name;") {
		t.Errorf("required provider `name` not emitted into config as `name = _cfg_name;`:\n%s", out)
	}
	if !regexp.MustCompile(`if cfg_overrides == null then \{\} else \{ overrides = cfg_overrides; \}`).MatchString(out) {
		t.Errorf("optional provider `overrides` not emitted into config under its real key from the alias:\n%s", out)
	}
	// A non-colliding attribute is unchanged (no alias).
	if !regexp.MustCompile(`if label == null then \{\} else \{ label = label; \}`).MatchString(out) {
		t.Errorf("non-colliding `label` should not be aliased:\n%s", out)
	}
	// Deterministic: same input, same output.
	if Emit("p", r) != out {
		t.Error("Emit is not deterministic for the same input")
	}
}

// A resource without collisions is emitted exactly as before (no aliasing).
func TestEmitNoCollisionUnchanged(t *testing.T) {
	r := Resource{
		Type: "token",
		Attrs: []Attr{
			{Name: "id", Computed: true, Type: NixType{Kind: KindString}},
			{Name: "label", Optional: true, Type: NixType{Kind: KindString}},
		},
	}
	out := Emit("alpha", r)
	if strings.Contains(out, "cfg_") {
		t.Errorf("a collision-free resource should have no cfg_ alias:\n%s", out)
	}
	if !strings.Contains(lambdaHead(t, out), "label ? null") {
		t.Errorf("optional `label` not emitted as a plain formal:\n%s", out)
	}
}

// lambdaHead returns the second lambda line ("{ name, ... }:") of the emitted
// module (after the "{ nivis }:" line), where formals are declared.
func lambdaHead(t *testing.T, out string) string {
	t.Helper()
	for _, line := range strings.Split(out, "\n") {
		s := strings.TrimSpace(line)
		if strings.HasPrefix(s, "{ name") {
			return s
		}
	}
	t.Fatalf("no `{ name ... }:` lambda head in:\n%s", out)
	return ""
}

// countFormal counts how many times `name` appears as a standalone formal in the
// lambda head (a comma/brace-delimited token), so `name` and `cfg_name` differ.
func countFormal(head, name string) int {
	re := regexp.MustCompile(`(^|[{,]\s*)` + regexp.QuoteMeta(name) + `(\s*[,?}])`)
	return len(re.FindAllString(head, -1))
}
