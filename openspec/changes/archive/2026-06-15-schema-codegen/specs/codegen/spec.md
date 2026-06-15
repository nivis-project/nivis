# Spec delta: codegen

## ADDED Requirements

### Requirement: Fetch a provider's schema
The codegen SHALL spawn a provider binary, complete the tfprotov6 handshake, call
`GetProviderSchema`, and produce a normalized schema model mapping each resource
type to its attributes (name, type, required/optional/computed/sensitive).

#### Scenario: schema fetched from a fake provider
- GIVEN the built `provider-alpha` binary
- WHEN the codegen fetches its schema
- THEN the model contains resource type `alpha_token` with attributes `label`
  (optional) and `id`/`value` (computed).

### Requirement: Schema-to-Nix type mapping
The codegen SHALL map each attribute's tftype and role flags to a Nix type
descriptor: scalars (string/number/bool), collections (list/set/map), and nested
objects; with roles required, optional, computed, and sensitive. A computed-only
attribute SHALL be modeled as an output (not a constructor input).

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

### Requirement: Constructor emission with required throws and an override seam
The codegen SHALL emit, per resource type, a Nix constructor that requires the
required inputs (throwing a named error if absent), passes optional inputs
through when present, omits computed-only attributes from inputs, and accepts an
`overrides` argument merged last so users can adjust generated output.

#### Scenario: required input missing throws
- GIVEN a generated constructor for a type with required `from`
- WHEN it is called without `from`
- THEN evaluation throws an error naming `from`.

#### Scenario: overrides win
- GIVEN a generated constructor called with `overrides = { config.x = 1; }`
- WHEN evaluated
- THEN the produced resource's `config.x` is `1`, overriding any generated default.

### Requirement: Codegen command
The codegen SHALL be runnable as a command
`nixform-gen --provider <path> --out <dir>` (built with `go build`/`go run`,
e.g. `go run ./cmd/nixform-gen -- --provider <path> --out <dir>`), writing
`<provider>/<type>.nix` files for each resource type. Packaging it as a flake
`apps.gen` is network-gated (it requires nixpkgs `buildGoModule`, and the binary
cache is unreachable per CLAUDE.md §6) and is tracked separately.

#### Scenario: end-to-end generation against a fake provider
- GIVEN the `provider-alpha` binary
- WHEN `nixform-gen` runs with `--provider <path> --out <dir>`
- THEN `<dir>` contains a generated constructor file for `alpha_token` that, when
  imported, produces a valid `mkResource` for that type.
