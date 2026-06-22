// Copyright 2026 TechNative B.V. and the nivis authors
// SPDX-License-Identifier: Apache-2.0

// Package ir defines the nivis JSON IR (docs/IR-CONTRACT.md) and the
// executor's in-memory model: typed resource nodes, provider configs, meta-args,
// and the reference edge list. It ingests and validates IR before the executor
// touches a provider.
package ir

// SchemaVersion is the IR version this executor understands.
const SchemaVersion = 1

// --- wire types (mirror docs/ir-schema.json) --------------------------------

// Document is the top-level IR.
type Document struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Providers     map[string]ProviderConfig `json:"providers"`
	Resources     []Resource                `json:"resources"`
	DataSources   []DataSource              `json:"dataSources,omitempty"`
	Edges         []Edge                    `json:"edges"`
	NixConsumers  []NixConsumer             `json:"nixConsumers"`
	// Backend is the optional remote-state backend declaration (e.g. an s3
	// bucket/key/region). Static config: it must be known before evaluation, so it
	// carries no refs/unknowns. Absent => the local file store. The IR layer only
	// validates `type` present and no-refs; a specific backend interprets its keys.
	Backend map[string]interface{} `json:"backend,omitempty"`
}

// ProviderConfig is a provider declaration. Config is a raw attribute tree.
type ProviderConfig struct {
	Source string                 `json:"source"`
	Config map[string]interface{} `json:"config"`
}

// Resource is one concrete (already-expanded) resource.
type Resource struct {
	ID       string                 `json:"id"`
	Provider string                 `json:"provider"`
	Type     string                 `json:"type"`
	Name     string                 `json:"name"`
	Config   map[string]interface{} `json:"config"`
	Meta     *Meta                  `json:"meta,omitempty"`
	// IsData marks a datasource node (from the IR's `dataSources` array): it is
	// READ via the provider's ReadDataSource, never planned, applied, written to
	// state, or destroyed. Datasource nodes live in the same Graph.Nodes/Order as
	// resources so they share the readiness/fixpoint machinery; the driver
	// dispatches read-vs-apply on this flag.
	IsData bool `json:"-"`
}

// DataSource is one datasource node in the IR's `dataSources` array. It is read,
// not created: no meta/lifecycle.
type DataSource struct {
	ID       string                 `json:"id"`
	Provider string                 `json:"provider"`
	Type     string                 `json:"type"`
	Name     string                 `json:"name"`
	Config   map[string]interface{} `json:"config"`
}

// Meta holds meta-arguments. count/for_each are intentionally absent: expansion
// happens in Nix (validated by ingest).
type Meta struct {
	DependsOn []string   `json:"dependsOn,omitempty"`
	Lifecycle *Lifecycle `json:"lifecycle,omitempty"`
}

// Lifecycle mirrors the contract's lifecycle block.
type Lifecycle struct {
	PreventDestroy bool     `json:"preventDestroy,omitempty"`
	IgnoreChanges  []string `json:"ignoreChanges,omitempty"`
}

// Edge is an explicit dependency edge in the IR.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Via  string `json:"via"`
}

// NixConsumer is a value Nix computed from resource outputs (the round trip).
type NixConsumer struct {
	ID    string                 `json:"id"`
	Value map[string]interface{} `json:"value"`
}

// --- leaf reference forms (the special marker objects) ----------------------

// Ref is a `{ "__ref": { resource, path } }` leaf.
type Ref struct {
	Resource string        `json:"resource"`
	Path     []interface{} `json:"path"`
}

// Derived is a `{ "__derived": { inputs } }` leaf (Nix-computed; *->Nix).
type Derived struct {
	Inputs []string `json:"inputs"`
}

// SensitiveRef is a `{ "__sensitiveRef": { resource, path } }` leaf.
type SensitiveRef struct {
	Resource string        `json:"resource"`
	Path     []interface{} `json:"path"`
}

// --- in-memory model --------------------------------------------------------

// RefClass classifies a reference by how it can be resolved.
type RefClass int

const (
	// ClassTFTF: a `__ref` inside a resource config; resolvable in-executor.
	ClassTFTF RefClass = iota
	// ClassStarToNix: a `__derived` leaf, or any leaf inside a nixConsumer;
	// resolvable only by Nix re-evaluation.
	ClassStarToNix
)

func (c RefClass) String() string {
	if c == ClassTFTF {
		return "TF->TF"
	}
	return "*->Nix"
}

// RefEdge is a classified reference discovered during ingest: a leaf at LeafPath
// within Owner that depends on a target resource (Ref/SensitiveRef) or on
// derived inputs (Derived).
type RefEdge struct {
	Owner string // owning resource id or consumer id
	// LeafPath is where the marker leaf sits in the owner's config/value tree
	// (used to substitute the resolved value back in).
	LeafPath []string
	Class    RefClass
	Target   string // target resource id for __ref/__sensitiveRef ("" for derived)
	// TargetPath is the path INTO the target resource's output object (the
	// __ref/__sensitiveRef "path"); nil for derived leaves.
	TargetPath []interface{}
	Inputs     []string // "<id>.<attr>" inputs for __derived (nil otherwise)
	Sensitive  bool     // true for __sensitiveRef leaves
}

// ResourceNode is the executor's working view of a resource.
type ResourceNode struct {
	Resource Resource
	// Refs are the TF->TF and derived references found within this resource's
	// config (derived refs are kept for pending-tracking but not resolved here).
	Refs []RefEdge
}

// Graph is the ingested, validated IR ready for the executor.
type Graph struct {
	Providers map[string]ProviderConfig
	Nodes     map[string]*ResourceNode // keyed by resource id
	Order     []string                 // resource ids in IR order (determinism)
	Edges     []Edge                   // explicit IR edges
	Consumers []NixConsumer
	// AllRefs is every classified reference across resources and consumers.
	AllRefs []RefEdge
	// Backend is the optional remote-state backend declaration (nil => local file
	// store). Validated static (type present, no refs) at ingest; its keys are
	// interpreted by the chosen backend, not here.
	Backend map[string]interface{}
}
