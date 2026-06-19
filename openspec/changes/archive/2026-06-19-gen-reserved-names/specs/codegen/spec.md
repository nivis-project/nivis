# Spec delta: codegen

## MODIFIED Requirements

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

The constructor reserves the parameter names `name` (the Nivis resource instance
name), `overrides`, and `nivis` for its own use. For any input attribute or block
whose name collides with a reserved parameter, the codegen SHALL accept it in the
lambda under a distinct safe alias (so no formal is duplicated and no reserved
parameter is shadowed) and SHALL emit it into `config` under its real attribute
key. The reserved `name` parameter SHALL continue to be the instance name threaded
to `mkResource`; a provider attribute named `name` SHALL appear in `config` as
`name`, distinct from the instance name. The generated `.nix` SHALL therefore
always be valid (no duplicate formal argument) regardless of the provider's
attribute names.

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

#### Scenario: a provider attribute named `name` does not collide with the instance name
- GIVEN a resource whose schema has an attribute literally named `name`
- WHEN the constructor is emitted
- THEN the lambda has the reserved `name` formal exactly once (the instance name), the provider attribute is accepted under a distinct alias, the generated `.nix` evaluates without a "duplicate formal argument" error, and the produced resource carries the instance name in its id and the provider `name` value in its `config`.

#### Scenario: a provider attribute named `overrides` is preserved
- GIVEN a resource whose schema has an attribute named `overrides`
- WHEN the constructor is emitted and evaluated
- THEN the `overrides` merge seam still works (it is not corrupted by the attribute), and the provider `overrides` attribute appears in `config` under its real key.
