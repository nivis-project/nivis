# Spec delta: nix-lib

## ADDED Requirements

### Requirement: mkProvider constructs a validated provider declaration
`mkProvider` SHALL accept `{ source, config ? {} }` and return the IR provider
shape `{ source, config }`, raising a named error if `source` is absent or not a
string. Nested provider blocks (e.g. `default_tags`, `assume_role`, `endpoints`)
SHALL be expressed as ordinary nested attrsets/lists inside `config`; `toIR`
serializes them and the executor's object-type construction maps them to the
provider schema. `mkProvider` is the ergonomic front door; `toIR` SHALL continue
to accept a bare `{ source, config }` attrset for backward compatibility.

#### Scenario: a provider config flows from Nix to the IR
- GIVEN `providers.aws = mkProvider { source = "registry.opentofu.org/hashicorp/aws"; config = { region = "eu-central-1"; }; }`
- WHEN `toIR` is evaluated
- THEN `providers.aws.source` is the given source and `providers.aws.config.region` is `"eu-central-1"`.

#### Scenario: missing source is a named error
- WHEN `mkProvider { config = {}; }` is evaluated (no `source`)
- THEN evaluation fails with an error naming `mkProvider` and the missing `source`.

## MODIFIED Requirements

### Requirement: toIR produces contract-conforming IR
`toIR` SHALL serialize providers, resources, edges, and nixConsumers to the
canonical IR with `schemaVersion: 1`; refs and derived leaves encoded as above;
edges derived from `__ref` usage. Each provider's `config` SHALL be resolved
against the outputs ledger and encoded with the **same** two passes as resource
config, so a `__ref`/`__derived` in provider config resolves each phase and
encodes to wire shape (and plain values pass through unchanged); `source` passes
through verbatim. The output SHALL conform to `docs/ir-schema.json` and the
referential rules.

#### Scenario: output validates against the schema
- GIVEN a resource set with one ref between two resources
- WHEN `toIR` is evaluated and the JSON is checked by `tests/ir-conformance/check.py`
- THEN it validates (structural + referential) with no error.

#### Scenario: edges reflect references
- GIVEN B.config.from references A
- WHEN `toIR` runs
- THEN the IR contains an edge `{ from: A, to: B, via: "from" }`.

#### Scenario: provider config resolves against the ledger
- GIVEN a provider whose `config` contains a `__derived` value built from a resource output, and a ledger holding that output
- WHEN `toIR` runs with that ledger
- THEN the provider's `config` in the IR holds the concrete derived value, not the placeholder.
