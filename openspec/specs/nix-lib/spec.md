# Spec: nix-lib

## Purpose
The nixform Nix library is the configuration frontend: users describe resources
as Nix values and the library serializes them to the canonical JSON IR the Go
executor consumes. It is the input side of the round trip — how a Nix author
references apply-time provider outputs (`__ref`) and computes values from them
(`__derived`), and how the flake `plan` interface re-resolves those against the
outputs ledger on each phase so the phased-eval loop (E3.5) converges. Output
conforms to `docs/ir-schema.json`, the same artifact the executor validates.
## Requirements
### Requirement: Resource construction with stable identity
`mkResource` SHALL accept `{ provider, type, name, config }` and return an
attrset with a stable id `<provider>.<type>.<name>` and a way to reference the
resource's (apply-time) output attributes as Nix values.

#### Scenario: id is derived from coordinates
- WHEN `mkResource { provider = "alpha"; type = "alpha_token"; name = "A"; config = {}; }` is evaluated
- THEN its id is `"alpha.alpha_token.A"`.

#### Scenario: output attribute access yields a ref
- GIVEN a resource A built with `mkResource`
- WHEN `A.value` (an output attribute) is accessed at eval time
- THEN it evaluates to a typed reference placeholder, not an error.

### Requirement: Reference encoding
Accessing an output attribute SHALL produce a `__ref` leaf
`{ "__ref": { "resource": <id>, "path": [<attr>...] } }`; a value computed *from*
an output (e.g. string interpolation) SHALL produce a `__derived` leaf
`{ "__derived": { "inputs": [<id.attr>...] } }`.

#### Scenario: direct reference is a __ref
- GIVEN B.config.from = A.value
- WHEN serialized at phase 0 (A unresolved)
- THEN the leaf is `{ "__ref": { "resource": "alpha.alpha_token.A", "path": ["value"] } }`.

#### Scenario: computed value is a __derived
- GIVEN name = "rec-" + A.value
- WHEN serialized at phase 0
- THEN the leaf is `{ "__derived": { "inputs": ["alpha.alpha_token.A.value"] } }`.

### Requirement: toIR produces contract-conforming IR
`toIR` SHALL serialize providers, resources, edges, and nixConsumers to the
canonical IR with `schemaVersion: 1`; refs and derived leaves encoded as above;
edges derived from `__ref` usage. The output SHALL conform to
`docs/ir-schema.json` and the referential rules.

#### Scenario: output validates against the schema
- GIVEN a resource set with one ref between two resources
- WHEN `toIR` is evaluated and the JSON is checked by `tests/ir-conformance/check.py`
- THEN it validates (structural + referential) with no error.

#### Scenario: edges reflect references
- GIVEN B.config.from references A
- WHEN `toIR` runs
- THEN the IR contains an edge `{ from: A, to: B, via: "from" }`.

### Requirement: Plan interface accepts the outputs ledger
The flake `nixform.plan` SHALL be a function of an injected outputs ledger
(`{ phase, outputs }`, empty on phase 0) and SHALL resolve `__ref`/`__derived`
leaves whose inputs are present in the ledger to concrete values, leaving the
rest as placeholders.

#### Scenario: phase 0 leaves refs unresolved
- WHEN `plan` is evaluated with an empty ledger
- THEN `__ref`/`__derived` leaves depending on unknown outputs remain placeholders.

#### Scenario: ledger resolves a derived value
- GIVEN name = "rec-" + A.value and a ledger with `A.value = "v0"`
- WHEN `plan` is re-evaluated with that ledger injected
- THEN `name` resolves to the concrete string `"rec-v0"` in the IR (no longer a `__derived` leaf).

