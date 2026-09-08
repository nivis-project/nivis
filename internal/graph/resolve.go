// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"sort"

	"github.com/nivis-project/nivis/internal/ir"
)

// Outputs maps a resource id to its known output attributes.
//
//	outputs["alpha.alpha_token.A"]["value"] = "x"
type Outputs map[string]map[string]interface{}

// BuildOutput identifies one build-output (`__build`) leaf found in a config: the
// OUTPUT path a provider is given, and the DERIVATION that produces it (empty
// for a leaf emitted by a Nix library predating that field). Named for the leaf
// rather than "Build", which in this package is the DAG constructor.
type BuildOutput struct {
	Path string
	Drv  string
}

// ResolveResult reports, after a resolution pass, which resources became fully
// known (all TF->TF refs resolved, no derived leaves) and which remain pending.
type ResolveResult struct {
	// Configs are deep copies of each resource's config with resolved TF->TF
	// refs substituted in place, and every `__build` leaf replaced by its output
	// path. Derived / *->Nix leaves are left untouched.
	Configs map[string]map[string]interface{}
	// Builds lists, per node id, the build outputs that node's config referenced.
	//
	// It is reported because substitution REMOVES the leaves: a consumer that
	// must build them (only the apply path does) cannot rediscover them from the
	// config afterwards. Sorted by output path, so a caller sees a stable order.
	BuildOutputs map[string][]BuildOutput
	// FullyKnown lists resource ids with no remaining unresolved refs of any
	// kind (ready for the provider).
	FullyKnown []string
	// Pending lists resource ids that still have an unresolved TF->TF ref or any
	// derived leaf (the latter only resolve via Nix re-eval, never here).
	Pending []string
}

// ResolveTFTF walks every node config (resources AND datasources), substituting
// known outputs into TF->TF (`__ref`) leaves and replacing every `__build` leaf
// with its output path. A resource is FullyKnown when every TF->TF ref it
// contains is resolved AND it has no `__derived` leaf; otherwise Pending.
//
// Build substitution happens HERE, not in each consumer, because it is a pure
// rewrite of a config this function already deep-copies — and because every
// consumer of a resolved config needs it. A provider's config encoder expects
// the value a leaf stands for, so a leaf that survives to a plan, an apply or a
// datasource read fails the operation with a type error naming an attribute.
// Doing it once means no code path can produce a config that still carries one;
// doing it per consumer meant one of five paths did (see nixform2-hytv).
//
// Building the outputs is a separate, effectful duty that belongs only to the
// apply path: see ResolveResult.Builds.
func ResolveTFTF(g *ir.Graph, outputs Outputs) ResolveResult {
	res := ResolveResult{
		Configs:      map[string]map[string]interface{}{},
		BuildOutputs: map[string][]BuildOutput{},
	}

	for _, id := range g.Order {
		n := g.Nodes[id]
		cfg := deepCopyMap(n.Resource.Config)

		pending := false
		for _, e := range n.Refs {
			switch e.Class {
			case ir.ClassStarToNix:
				// derived leaf inside a config: resolvable only by re-eval.
				pending = true
			case ir.ClassTFTF:
				val, ok := lookupOutput(outputs, e.Target, e.TargetPath)
				if !ok {
					pending = true
					continue
				}
				setAtPath(cfg, e.LeafPath, val)
			}
		}

		if builds := substituteBuilds(cfg); len(builds) > 0 {
			res.BuildOutputs[id] = builds
		}

		res.Configs[id] = cfg
		if pending {
			res.Pending = append(res.Pending, id)
		} else {
			res.FullyKnown = append(res.FullyKnown, id)
		}
	}
	return res
}

// lookupOutput resolves a ref's TargetPath against known outputs. Path elements
// are strings (map keys) or numbers (list indices, JSON-decoded as float64). The
// first element selects the output attribute; deeper elements descend.
func lookupOutput(outputs Outputs, target string, path []interface{}) (interface{}, bool) {
	attrs, ok := outputs[target]
	if !ok || len(path) == 0 {
		return nil, false
	}
	key, ok := path[0].(string)
	if !ok {
		return nil, false
	}
	cur, ok := attrs[key]
	if !ok {
		return nil, false
	}
	for _, p := range path[1:] {
		cur, ok = index(cur, p)
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// index descends one level into a nested value by a path element: a string maps
// to an object key; a number (or numeric value) indexes an array.
func index(v interface{}, key interface{}) (interface{}, bool) {
	switch t := v.(type) {
	case map[string]interface{}:
		s, ok := key.(string)
		if !ok {
			return nil, false
		}
		x, ok := t[s]
		return x, ok
	case []interface{}:
		i, ok := asIndex(key)
		if !ok || i < 0 || i >= len(t) {
			return nil, false
		}
		return t[i], true
	}
	return nil, false
}

// asIndex coerces a path element to an array index. JSON numbers decode to
// float64; some producers may emit an int or a numeric string.
func asIndex(key interface{}) (int, bool) {
	switch k := key.(type) {
	case float64:
		return int(k), true
	case int:
		return k, true
	case string:
		if i := atoi(k); i >= 0 {
			return i, true
		}
	}
	return 0, false
}

// setAtPath sets cfg at the given path (string keys; numeric strings for array
// indices) to val. Intermediate containers are assumed to exist (they came from
// the same config tree the path was discovered in).
func setAtPath(cfg map[string]interface{}, path []string, val interface{}) {
	if len(path) == 0 {
		return
	}
	if len(path) == 1 {
		cfg[path[0]] = val
		return
	}
	var cur interface{} = cfg
	for _, key := range path[:len(path)-1] {
		next, ok := index(cur, key)
		if !ok {
			return
		}
		cur = next
	}
	last := path[len(path)-1]
	switch t := cur.(type) {
	case map[string]interface{}:
		t[last] = val
	case []interface{}:
		if i := atoi(last); i >= 0 && i < len(t) {
			t[i] = val
		}
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	if s == "" {
		return -1
	}
	return n
}

func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(m))
	for k, v := range m {
		out[k] = deepCopyValue(v)
	}
	return out
}

func deepCopyValue(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return deepCopyMap(t)
	case []interface{}:
		out := make([]interface{}, len(t))
		for i, x := range t {
			out[i] = deepCopyValue(x)
		}
		return out
	default:
		return v
	}
}

// substituteBuilds replaces every `__build` leaf in cfg with its output path
// string, in place, and returns the builds it found sorted by that path.
//
// cfg is already a deep copy, so mutating it cannot affect the ingested graph.
// The walk performs no I/O: it neither checks whether a path exists nor builds
// anything, which is what makes it safe on the plan and datasource-read paths.
func substituteBuilds(cfg map[string]interface{}) []BuildOutput {
	var found []BuildOutput
	for k, v := range cfg {
		cfg[k] = substituteBuildValue(v, &found)
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Path < found[j].Path })
	return found
}

// substituteBuildValue rewrites one value, recursing through maps and slices.
func substituteBuildValue(v interface{}, found *[]BuildOutput) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		if b, ok := buildLeaf(t); ok {
			*found = append(*found, b)
			return b.Path // the leaf becomes the path string it stands for
		}
		for k, child := range t {
			t[k] = substituteBuildValue(child, found)
		}
		return t
	case []interface{}:
		for i, child := range t {
			t[i] = substituteBuildValue(child, found)
		}
		return t
	default:
		return v
	}
}

// buildLeaf reads a `__build` leaf: its output path and, when the emitting Nix
// library was new enough to record it, the derivation that produces it. A map
// that is not a well-formed leaf is left alone (it is ordinary nested config).
func buildLeaf(m map[string]interface{}) (BuildOutput, bool) {
	raw, ok := m["__build"].(map[string]interface{})
	if !ok {
		return BuildOutput{}, false
	}
	path, ok := raw["path"].(string)
	if !ok || path == "" {
		return BuildOutput{}, false
	}
	drv, _ := raw["drv"].(string)
	return BuildOutput{Path: path, Drv: drv}, true
}
