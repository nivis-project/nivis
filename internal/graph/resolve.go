// Copyright 2026 WeareTechnative B.V. and the terrae-nivis authors
// SPDX-License-Identifier: Apache-2.0

package graph

import (
	"github.com/wearetechnative/terrae-nivis/internal/ir"
)

// Outputs maps a resource id to its known output attributes.
//
//	outputs["alpha.alpha_token.A"]["value"] = "x"
type Outputs map[string]map[string]interface{}

// ResolveResult reports, after a resolution pass, which resources became fully
// known (all TF->TF refs resolved, no derived leaves) and which remain pending.
type ResolveResult struct {
	// Configs are deep copies of each resource's config with resolved TF->TF
	// refs substituted in place. Derived / *->Nix leaves are left untouched.
	Configs map[string]map[string]interface{}
	// FullyKnown lists resource ids with no remaining unresolved refs of any
	// kind (ready for the provider).
	FullyKnown []string
	// Pending lists resource ids that still have an unresolved TF->TF ref or any
	// derived leaf (the latter only resolve via Nix re-eval, never here).
	Pending []string
}

// ResolveTFTF walks every resource config, substituting known outputs into
// TF->TF (`__ref`) leaves. A resource is FullyKnown when every TF->TF ref it
// contains is resolved AND it has no `__derived` leaf; otherwise Pending.
func ResolveTFTF(g *ir.Graph, outputs Outputs) ResolveResult {
	res := ResolveResult{Configs: map[string]map[string]interface{}{}}

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
