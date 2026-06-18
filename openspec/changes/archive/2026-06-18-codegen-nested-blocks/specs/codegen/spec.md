# Spec delta: codegen

## MODIFIED Requirements

### Requirement: Schema-to-Nix type mapping
The codegen SHALL map each attribute's tftype and role flags to a Nix type
descriptor: scalars (string/number/bool), collections (list/set/map), and nested
objects; with roles required, optional, computed, and sensitive. A computed-only
attribute SHALL be modeled as an output (not a constructor input). The codegen
SHALL additionally model a resource's **nested blocks** (the schema's nested
block types): each block has a name, a nesting mode (single, list, set, or map),
and its own inner attributes (recursively, for blocks nested within blocks). A
block is a constructor input (blocks are author-supplied configuration).

#### Scenario: scalar roles
- GIVEN a required string `from` and a computed string `endpoint`
- WHEN mapped
- THEN `from` is a required string input and `endpoint` is a computed output (no input arg).

#### Scenario: collections and nested objects
- GIVEN a `list(string)` attr, a `map(number)` attr, and a nested object attr
- WHEN mapped
- THEN each maps to the corresponding Nix type descriptor (list/map/object) preserving the element/attribute types.

#### Scenario: sensitive is flagged
- GIVEN an attribute marked sensitive
- WHEN mapped
- THEN the descriptor records it as sensitive so the emitter can handle it per the IR contract.

#### Scenario: nested blocks are modeled with their nesting mode
- GIVEN a resource with a list-nested block `ingress` and a single-nested block `x`, each with inner attributes
- WHEN mapped
- THEN the model records `ingress` as a list-nested block and `x` as a single-nested block, each carrying its inner attributes.

### Requirement: Constructor emission with required throws and an override seam
The codegen SHALL emit, per resource type, a Nix constructor that requires the
required inputs (throwing a named error if absent), passes optional inputs
through when present, omits computed-only attributes from inputs, and accepts an
`overrides` argument merged last so users can adjust generated output. The
constructor SHALL also expose each **nested block** as an input whose default
shape matches its nesting, so the correct shape is the obvious one and the
list-vs-single mistake cannot be made by guessing:
- a list-nested or set-nested block defaults to `[]` (a list of attrsets),
- a single-nested block defaults to `null` (one attrset),
- a map-nested block defaults to `{}` (a map of attrsets).
Each block SHALL be documented in the generated file with its nesting and its
inner attribute names, so the generated constructor doubles as the per-provider
argument reference.

#### Scenario: required input missing throws
- GIVEN a generated constructor for a type with required `from`
- WHEN it is called without `from`
- THEN evaluation throws an error naming `from`.

#### Scenario: overrides win
- GIVEN a generated constructor called with `overrides = { config.x = 1; }`
- WHEN evaluated
- THEN the produced resource's `config.x` is `1`, overriding any generated default.

#### Scenario: a list-nested block is emitted with a list shape and documented
- GIVEN a resource with a list-nested block `ingress`
- WHEN the constructor is emitted
- THEN `ingress` is a constructor argument defaulting to `[]`, and the generated file documents it as list-nested with its inner attributes (so the author writes `[ { ... } ]`, not a bare attrset).

#### Scenario: a single-nested block is emitted as an attrset
- GIVEN a resource with a single-nested block `x`
- WHEN the constructor is emitted
- THEN `x` is a constructor argument defaulting to `null` and documented as a single attrset.
