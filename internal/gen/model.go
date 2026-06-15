// Package gen turns a provider's tfprotov6 schema into typed Nix constructors
// (nixform-gen). It is the path to "all providers with zero per-provider work"
// (DESIGN D2): generic, schema-driven, no per-provider code.
package gen

// Kind is a Nix type kind in the codegen's type model.
type Kind string

const (
	KindString Kind = "string"
	KindNumber Kind = "number"
	KindBool   Kind = "bool"
	KindList   Kind = "list"
	KindSet    Kind = "set"
	KindMap    Kind = "map"
	KindObject Kind = "object"
	// KindDynamic is a fallback for tftypes we don't model precisely.
	KindDynamic Kind = "dynamic"
)

// NixType describes an attribute's type. Elem is set for list/set/map; Attrs for
// object.
type NixType struct {
	Kind  Kind
	Elem  *NixType            // list/set/map element type
	Attrs map[string]*NixType // object attribute types
}

// Attr is one resource attribute in the normalized model.
type Attr struct {
	Name      string
	Type      NixType
	Required  bool
	Optional  bool
	Computed  bool
	Sensitive bool
}

// IsInput reports whether the attribute is a constructor input. A computed-only
// attribute (computed && !required && !optional) is an output, not an input.
func (a Attr) IsInput() bool { return a.Required || a.Optional }

// Resource is a resource type's normalized schema.
type Resource struct {
	Type  string
	Attrs []Attr // sorted by name for deterministic emission
}
