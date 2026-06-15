// Copyright 2026 WeareTechnative B.V. and the nixform authors
// SPDX-License-Identifier: Apache-2.0

package ir

import "fmt"

// leafKind inspects a decoded JSON value and, if it is one of the reserved
// marker objects, returns the parsed form. A plain object/array/scalar returns
// kind "" with the original value.
func parseLeaf(v interface{}) (kind string, ref *Ref, der *Derived, sref *SensitiveRef, err error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return "", nil, nil, nil, nil
	}
	switch {
	case has(m, "__ref"):
		r, e := decodeRef(m["__ref"])
		return "__ref", r, nil, nil, e
	case has(m, "__derived"):
		d, e := decodeDerived(m["__derived"])
		return "__derived", nil, d, nil, e
	case has(m, "__sensitiveRef"):
		s, e := decodeSensitiveRef(m["__sensitiveRef"])
		return "__sensitiveRef", nil, nil, s, e
	}
	return "", nil, nil, nil, nil
}

func has(m map[string]interface{}, k string) bool {
	_, ok := m[k]
	return ok
}

func decodeRef(v interface{}) (*Ref, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("__ref must be an object")
	}
	res, _ := m["resource"].(string)
	if res == "" {
		return nil, fmt.Errorf("__ref missing 'resource'")
	}
	path, ok := m["path"].([]interface{})
	if !ok || len(path) == 0 {
		return nil, fmt.Errorf("__ref for resource %q missing non-empty 'path'", res)
	}
	return &Ref{Resource: res, Path: path}, nil
}

func decodeSensitiveRef(v interface{}) (*SensitiveRef, error) {
	r, err := decodeRef(v)
	if err != nil {
		return nil, err
	}
	return &SensitiveRef{Resource: r.Resource, Path: r.Path}, nil
}

func decodeDerived(v interface{}) (*Derived, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("__derived must be an object")
	}
	raw, ok := m["inputs"].([]interface{})
	if !ok || len(raw) == 0 {
		return nil, fmt.Errorf("__derived missing non-empty 'inputs'")
	}
	inputs := make([]string, 0, len(raw))
	for _, x := range raw {
		s, ok := x.(string)
		if !ok {
			return nil, fmt.Errorf("__derived input must be a string, got %T", x)
		}
		inputs = append(inputs, s)
	}
	return &Derived{Inputs: inputs}, nil
}

// walkRefs walks a config/value tree, invoking fn for each marker leaf it finds,
// with the path to that leaf. Recurses into plain objects and arrays. Returns
// the first error fn or leaf-parsing produces.
func walkRefs(tree map[string]interface{}, fn func(path []string, kind string, ref *Ref, der *Derived, sref *SensitiveRef) error) error {
	return walkNode(nil, tree, fn)
}

func walkNode(path []string, v interface{}, fn func([]string, string, *Ref, *Derived, *SensitiveRef) error) error {
	kind, ref, der, sref, err := parseLeaf(v)
	if err != nil {
		return fmt.Errorf("at path %v: %w", path, err)
	}
	if kind != "" {
		return fn(path, kind, ref, der, sref)
	}
	switch t := v.(type) {
	case map[string]interface{}:
		for k, child := range t {
			if err := walkNode(append(append([]string{}, path...), k), child, fn); err != nil {
				return err
			}
		}
	case []interface{}:
		for i, child := range t {
			if err := walkNode(append(append([]string{}, path...), fmt.Sprintf("%d", i)), child, fn); err != nil {
				return err
			}
		}
	}
	return nil
}

// classifyConfig finds the references inside a resource config. A `__ref` is
// TF->TF; a `__derived` is *->Nix; a `__sensitiveRef` is treated as TF->TF for
// graph purposes (it points at a resource output) but flagged sensitive by the
// caller via Target.
func classifyConfig(owner string, cfg map[string]interface{}) ([]RefEdge, error) {
	var edges []RefEdge
	err := walkRefs(cfg, func(path []string, kind string, ref *Ref, der *Derived, sref *SensitiveRef) error {
		switch kind {
		case "__ref":
			edges = append(edges, RefEdge{Owner: owner, LeafPath: path, Class: ClassTFTF, Target: ref.Resource, TargetPath: ref.Path})
		case "__sensitiveRef":
			edges = append(edges, RefEdge{Owner: owner, LeafPath: path, Class: ClassTFTF, Target: sref.Resource, TargetPath: sref.Path, Sensitive: true})
		case "__derived":
			edges = append(edges, RefEdge{Owner: owner, LeafPath: path, Class: ClassStarToNix, Inputs: der.Inputs})
		}
		return nil
	})
	return edges, err
}

// classifyConsumer finds references inside a nixConsumer value. Everything in a
// consumer is *->Nix (it is, by definition, a Nix-computed value).
func classifyConsumer(owner string, val map[string]interface{}) ([]RefEdge, error) {
	var edges []RefEdge
	err := walkRefs(val, func(path []string, kind string, ref *Ref, der *Derived, sref *SensitiveRef) error {
		e := RefEdge{Owner: owner, LeafPath: path, Class: ClassStarToNix}
		switch kind {
		case "__ref":
			e.Target, e.TargetPath = ref.Resource, ref.Path
		case "__sensitiveRef":
			e.Target, e.TargetPath, e.Sensitive = sref.Resource, sref.Path, true
		case "__derived":
			e.Inputs = der.Inputs
		}
		edges = append(edges, e)
		return nil
	})
	return edges, err
}
