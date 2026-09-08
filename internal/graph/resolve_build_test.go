// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package graph

// Build-output substitution during resolution: the leaf becomes its path for
// EVERY consumer of a resolved config, and what was substituted is reported so
// the apply path knows what to build.
//
// This is where the substitution lives because it is pure. Putting it in each
// consumer is what produced nixform2-hytv: one of five paths did it, and the
// other four handed a raw leaf to a provider's config encoder.

import (
	"reflect"
	"testing"

	"github.com/nivis-project/nivis/internal/ir"
)

// leaf is a __build leaf carrying an output path and, optionally, a derivation.
func leaf(path, drv string) map[string]interface{} {
	b := map[string]interface{}{"path": path}
	if drv != "" {
		b["drv"] = drv
	}
	return map[string]interface{}{"__build": b}
}

// graphWith builds a one-node graph (a resource, or a datasource) with the config.
func graphWith(id string, isData bool, cfg map[string]interface{}) *ir.Graph {
	return &ir.Graph{
		Providers: map[string]ir.ProviderConfig{"alpha": {Source: "provider-alpha"}},
		Nodes: map[string]*ir.ResourceNode{
			id: {Resource: ir.Resource{ID: id, Provider: "alpha", Type: "t", Name: "n", Config: cfg, IsData: isData}},
		},
		Order: []string{id},
	}
}

// A leaf is replaced by its path at any depth, and reported with both paths.
func TestResolveSubstitutesBuildLeaves(t *testing.T) {
	cfg := map[string]interface{}{
		"source": leaf("/nix/store/aaa-img/x.vhd", "/nix/store/ddd-img.drv"),
		"nested": map[string]interface{}{
			"inner": leaf("/nix/store/bbb-pkg", "/nix/store/eee-pkg.drv"),
		},
		"list": []interface{}{
			leaf("/nix/store/ccc-thing", "/nix/store/fff-thing.drv"),
			"plain",
		},
		"scalar": "untouched",
	}
	res := ResolveTFTF(graphWith("alpha.t.n", false, cfg), Outputs{})
	got := res.Configs["alpha.t.n"]

	if got["source"] != "/nix/store/aaa-img/x.vhd" {
		t.Errorf("top-level leaf not substituted: %#v", got["source"])
	}
	if inner := got["nested"].(map[string]interface{})["inner"]; inner != "/nix/store/bbb-pkg" {
		t.Errorf("nested leaf not substituted: %#v", inner)
	}
	if first := got["list"].([]interface{})[0]; first != "/nix/store/ccc-thing" {
		t.Errorf("leaf in a list not substituted: %#v", first)
	}
	if got["scalar"] != "untouched" {
		t.Errorf("plain value changed: %#v", got["scalar"])
	}

	// Reported, sorted by output path, with the derivation each needs to build.
	want := []BuildOutput{
		{Path: "/nix/store/aaa-img/x.vhd", Drv: "/nix/store/ddd-img.drv"},
		{Path: "/nix/store/bbb-pkg", Drv: "/nix/store/eee-pkg.drv"},
		{Path: "/nix/store/ccc-thing", Drv: "/nix/store/fff-thing.drv"},
	}
	if !reflect.DeepEqual(res.BuildOutputs["alpha.t.n"], want) {
		t.Errorf("BuildOutputs = %v, want %v", res.BuildOutputs["alpha.t.n"], want)
	}
}

// A DATASOURCE config gets the same treatment: it flows through the same resolve
// pass, which is why fixing this here covers plan, apply and output at once.
func TestResolveSubstitutesInDatasourceConfigs(t *testing.T) {
	cfg := map[string]interface{}{"query": leaf("/nix/store/aaa-img", "/nix/store/ddd.drv")}
	res := ResolveTFTF(graphWith("data.alpha.t.n", true, cfg), Outputs{})

	if q := res.Configs["data.alpha.t.n"]["query"]; q != "/nix/store/aaa-img" {
		t.Errorf("datasource leaf not substituted: %#v", q)
	}
	if len(res.BuildOutputs["data.alpha.t.n"]) != 1 {
		t.Errorf("the datasource's build was not reported: %v", res.BuildOutputs)
	}
}

// A legacy leaf (no derivation) still substitutes; the report says it has none,
// which is what lets the realiser explain why it cannot be built.
func TestResolveSubstitutesLegacyLeaf(t *testing.T) {
	cfg := map[string]interface{}{"source": leaf("/nix/store/aaa-img", "")}
	res := ResolveTFTF(graphWith("alpha.t.n", false, cfg), Outputs{})

	if got := res.Configs["alpha.t.n"]["source"]; got != "/nix/store/aaa-img" {
		t.Errorf("legacy leaf not substituted: %#v", got)
	}
	want := []BuildOutput{{Path: "/nix/store/aaa-img"}}
	if !reflect.DeepEqual(res.BuildOutputs["alpha.t.n"], want) {
		t.Errorf("BuildOutputs = %v, want %v", res.BuildOutputs["alpha.t.n"], want)
	}
}

// A config with no build outputs reports none, and nothing else is disturbed.
func TestResolveWithoutBuildLeaves(t *testing.T) {
	cfg := map[string]interface{}{
		"a": "x",
		"b": map[string]interface{}{"c": 1.0},
		"d": []interface{}{"e"},
	}
	res := ResolveTFTF(graphWith("alpha.t.n", false, cfg), Outputs{})

	if len(res.BuildOutputs) != 0 {
		t.Errorf("BuildOutputs should be empty, got %v", res.BuildOutputs)
	}
	if !reflect.DeepEqual(res.Configs["alpha.t.n"], cfg) {
		t.Errorf("config changed: %#v", res.Configs["alpha.t.n"])
	}
}

// Maps that merely look like leaves are ordinary config and are left alone.
func TestResolveIgnoresMalformedLeaves(t *testing.T) {
	cfg := map[string]interface{}{
		"noPath":      map[string]interface{}{"__build": map[string]interface{}{"drv": "/nix/store/x.drv"}},
		"emptyPath":   map[string]interface{}{"__build": map[string]interface{}{"path": ""}},
		"wrongType":   map[string]interface{}{"__build": map[string]interface{}{"path": 42.0}},
		"notAnObject": map[string]interface{}{"__build": "nope"},
		"otherKey":    map[string]interface{}{"build": map[string]interface{}{"path": "/nix/store/x"}},
	}
	res := ResolveTFTF(graphWith("alpha.t.n", false, cfg), Outputs{})

	if len(res.BuildOutputs) != 0 {
		t.Errorf("nothing should have been reported, got %v", res.BuildOutputs)
	}
	for k, v := range cfg {
		if !reflect.DeepEqual(res.Configs["alpha.t.n"][k], v) {
			t.Errorf("%s changed: %#v", k, res.Configs["alpha.t.n"][k])
		}
	}
}

// Substitution performs no I/O: paths that do not exist substitute fine, which is
// what makes it safe on the plan and datasource-read paths (they must not build).
func TestResolveSubstitutionDoesNoIO(t *testing.T) {
	cfg := map[string]interface{}{
		"source": leaf("/nix/store/does-not-exist-anywhere/x.vhd", "/nix/store/nor-does-this.drv"),
	}
	res := ResolveTFTF(graphWith("alpha.t.n", false, cfg), Outputs{})

	if got := res.Configs["alpha.t.n"]["source"]; got != "/nix/store/does-not-exist-anywhere/x.vhd" {
		t.Errorf("a non-existent path should still substitute: %#v", got)
	}
}

// Substitution does not disturb classification: a config whose only marker is a
// build leaf stays FullyKnown (a build leaf never depended on the ledger).
func TestResolveBuildLeafStaysFullyKnown(t *testing.T) {
	cfg := map[string]interface{}{"source": leaf("/nix/store/aaa-img", "/nix/store/ddd.drv")}
	res := ResolveTFTF(graphWith("alpha.t.n", false, cfg), Outputs{})

	if len(res.FullyKnown) != 1 || res.FullyKnown[0] != "alpha.t.n" {
		t.Errorf("FullyKnown = %v, want [alpha.t.n]", res.FullyKnown)
	}
	if len(res.Pending) != 0 {
		t.Errorf("Pending = %v, want none", res.Pending)
	}
}

// The ingested graph is not mutated: configs are deep copies, so substitution
// cannot leak into a later phase's view of the graph.
func TestResolveDoesNotMutateTheGraph(t *testing.T) {
	original := leaf("/nix/store/aaa-img", "/nix/store/ddd.drv")
	cfg := map[string]interface{}{"source": original}
	g := graphWith("alpha.t.n", false, cfg)

	ResolveTFTF(g, Outputs{})

	got := g.Nodes["alpha.t.n"].Resource.Config["source"]
	if _, stillALeaf := got.(map[string]interface{}); !stillALeaf {
		t.Errorf("the graph's own config was mutated: %#v", got)
	}
}
