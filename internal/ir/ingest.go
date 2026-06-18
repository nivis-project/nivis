// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

package ir

import (
	"encoding/json"
	"fmt"
	"strings"
)

// IngestIR unmarshals and validates an IR document against the contract, then
// builds the executor's working Graph. All validation errors name the offending
// resource, edge, or path (the contract's "rejected with identity" rule).
func IngestIR(data []byte) (*Graph, error) {
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("ir: invalid JSON: %w", err)
	}
	if doc.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("ir: unsupported schemaVersion %d (want %d)", doc.SchemaVersion, SchemaVersion)
	}

	// Second, raw pass to inspect meta objects for disallowed keys the typed
	// Meta struct would silently drop (count/for_each).
	rawMeta, err := rawResourceMetas(data)
	if err != nil {
		return nil, err
	}

	g := &Graph{
		Providers: doc.Providers,
		Nodes:     make(map[string]*ResourceNode, len(doc.Resources)),
		Edges:     doc.Edges,
		Consumers: doc.NixConsumers,
	}
	if g.Providers == nil {
		g.Providers = map[string]ProviderConfig{}
	}

	// Resources: unique ids, declared provider, no count/for_each, classify refs.
	for i := range doc.Resources {
		r := doc.Resources[i]
		if r.ID == "" {
			return nil, fmt.Errorf("ir: resource at index %d has empty id", i)
		}
		if _, dup := g.Nodes[r.ID]; dup {
			return nil, fmt.Errorf("ir: duplicate resource id %q", r.ID)
		}
		if _, ok := g.Providers[r.Provider]; !ok {
			return nil, fmt.Errorf("ir: resource %q uses undeclared provider %q", r.ID, r.Provider)
		}
		if err := checkNoExpansionMeta(r.ID, r.Config, rawMeta[r.ID]); err != nil {
			return nil, err
		}
		refs, err := classifyConfig(r.ID, r.Config)
		if err != nil {
			return nil, fmt.Errorf("ir: resource %q: %w", r.ID, err)
		}
		g.Nodes[r.ID] = &ResourceNode{Resource: r, Refs: refs}
		g.Order = append(g.Order, r.ID)
		g.AllRefs = append(g.AllRefs, refs...)
	}

	// Datasources: like resources but READ, not created. They live in the same
	// Nodes/Order (so they share the readiness/fixpoint machinery), marked
	// IsData; the driver dispatches read-vs-apply on the flag. Ids must be
	// "data."-prefixed, unique, and must not collide with a resource id.
	for i := range doc.DataSources {
		d := doc.DataSources[i]
		if d.ID == "" {
			return nil, fmt.Errorf("ir: dataSource at index %d has empty id", i)
		}
		if !strings.HasPrefix(d.ID, "data.") {
			return nil, fmt.Errorf("ir: dataSource id %q must begin with \"data.\"", d.ID)
		}
		if _, dup := g.Nodes[d.ID]; dup {
			return nil, fmt.Errorf("ir: dataSource id %q collides with an existing node id", d.ID)
		}
		if _, ok := g.Providers[d.Provider]; !ok {
			return nil, fmt.Errorf("ir: dataSource %q uses undeclared provider %q", d.ID, d.Provider)
		}
		refs, err := classifyConfig(d.ID, d.Config)
		if err != nil {
			return nil, fmt.Errorf("ir: dataSource %q: %w", d.ID, err)
		}
		r := Resource{ID: d.ID, Provider: d.Provider, Type: d.Type, Name: d.Name, Config: d.Config, IsData: true}
		g.Nodes[d.ID] = &ResourceNode{Resource: r, Refs: refs}
		g.Order = append(g.Order, d.ID)
		g.AllRefs = append(g.AllRefs, refs...)
	}

	// Ref targets must exist.
	for _, e := range g.AllRefs {
		if e.Target != "" {
			if _, ok := g.Nodes[e.Target]; !ok {
				return nil, fmt.Errorf("ir: %s ref in %q at path %v targets non-existent resource %q",
					e.Class, e.Owner, e.LeafPath, e.Target)
			}
		}
	}

	// Edge endpoints must exist.
	for _, e := range doc.Edges {
		if _, ok := g.Nodes[e.From]; !ok {
			return nil, fmt.Errorf("ir: edge from->%q (to=%q via=%q) names a non-existent resource", e.From, e.To, e.Via)
		}
		if _, ok := g.Nodes[e.To]; !ok {
			return nil, fmt.Errorf("ir: edge to->%q (from=%q via=%q) names a non-existent resource", e.To, e.From, e.Via)
		}
	}

	// depends_on targets must exist.
	for _, id := range g.Order {
		n := g.Nodes[id]
		if n.Resource.Meta == nil {
			continue
		}
		for _, dep := range n.Resource.Meta.DependsOn {
			if _, ok := g.Nodes[dep]; !ok {
				return nil, fmt.Errorf("ir: resource %q dependsOn non-existent resource %q", id, dep)
			}
		}
	}

	// Consumers: unique ids, classify (all *->Nix), targets exist.
	seenC := map[string]bool{}
	for _, c := range doc.NixConsumers {
		if c.ID == "" {
			return nil, fmt.Errorf("ir: nixConsumer with empty id")
		}
		if seenC[c.ID] {
			return nil, fmt.Errorf("ir: duplicate nixConsumer id %q", c.ID)
		}
		seenC[c.ID] = true
		refs, err := classifyConsumer(c.ID, c.Value)
		if err != nil {
			return nil, fmt.Errorf("ir: nixConsumer %q: %w", c.ID, err)
		}
		for _, e := range refs {
			if e.Target != "" {
				if _, ok := g.Nodes[e.Target]; !ok {
					return nil, fmt.Errorf("ir: nixConsumer %q at path %v targets non-existent resource %q",
						c.ID, e.LeafPath, e.Target)
				}
			}
		}
		g.AllRefs = append(g.AllRefs, refs...)
	}

	return g, nil
}

// checkNoExpansionMeta rejects count/for_each in a resource's config or meta:
// the contract requires expansion to have happened in Nix.
func checkNoExpansionMeta(id string, cfg, meta map[string]interface{}) error {
	for _, k := range []string{"count", "for_each"} {
		if has(cfg, k) {
			return fmt.Errorf("ir: resource %q config contains %q; expansion must happen in Nix", id, k)
		}
		if meta != nil && has(meta, k) {
			return fmt.Errorf("ir: resource %q meta contains %q; expansion must happen in Nix", id, k)
		}
	}
	return nil
}

// rawResourceMetas re-decodes the document just far enough to return each
// resource id's raw meta object, so disallowed keys can be detected.
func rawResourceMetas(data []byte) (map[string]map[string]interface{}, error) {
	var raw struct {
		Resources []struct {
			ID   string                 `json:"id"`
			Meta map[string]interface{} `json:"meta"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("ir: invalid JSON (meta pass): %w", err)
	}
	out := make(map[string]map[string]interface{}, len(raw.Resources))
	for _, r := range raw.Resources {
		out[r.ID] = r.Meta
	}
	return out, nil
}
