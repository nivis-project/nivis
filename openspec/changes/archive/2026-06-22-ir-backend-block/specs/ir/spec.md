# Spec delta: ir

## MODIFIED Requirements

### Requirement: Canonical IR shape
The system SHALL define a versioned JSON IR with top-level `schemaVersion`,
`providers`, `resources`, `edges`, and `nixConsumers`, and an OPTIONAL top-level
`backend` object, as specified in `docs/IR-CONTRACT.md`. The `backend` field, when
present, declares where state is stored; when absent, the executor uses the local
file store. Adding the optional `backend` field is additive and does not change
`schemaVersion`.

#### Scenario: well-formed IR validates
- GIVEN an IR with unique resource ids and every edge endpoint present
- WHEN it is validated by the Nix property test and Go `IngestIR`
- THEN both accept it without error.

#### Scenario: malformed IR is rejected with identity
- GIVEN an IR with an edge referencing a non-existent resource id
- WHEN `IngestIR` validates it
- THEN it fails with an error naming the offending edge and resource id.

#### Scenario: an IR without a backend validates and means the local store
- GIVEN an IR with no `backend` field
- WHEN it is ingested
- THEN it validates and the executor treats state as the local file store (unchanged behaviour).

## ADDED Requirements

### Requirement: Backend declaration in the IR
The IR SHALL support an optional top-level `backend` object declaring where state
is stored. The `backend`, when present, is **static configuration**: it SHALL be
known before any evaluation, so its leaves SHALL be plain JSON scalars/objects and
SHALL NOT contain a `__ref`, `__derived`, or unknown leaf. It SHALL have a
non-empty string `type` identifying the backend.
Other keys are backend-specific and are NOT interpreted by the IR layer (a
specific backend defines and validates its own keys). Credentials SHALL NOT appear
in `backend`; only the location of state. `toIR` SHALL emit `backend` only when the
config declares one, and omit it otherwise.

#### Scenario: a static backend parses
- GIVEN an IR whose `backend` is `{ "type": "s3", "bucket": "b", "key": "k", "region": "r" }`
- WHEN it is ingested
- THEN it validates and the backend object is available to the executor.

#### Scenario: a backend without a type is rejected
- GIVEN an IR whose `backend` omits `type` (or `type` is empty)
- WHEN it is ingested
- THEN ingestion fails with an actionable error stating `backend.type` is required.

#### Scenario: a backend containing a ref is rejected
- GIVEN an IR whose `backend` has a leaf encoded as a `__ref` or `__derived` (or any unknown)
- WHEN it is ingested
- THEN ingestion fails with an actionable error naming the offending path, because the backend must be statically known before evaluation.

#### Scenario: toIR emits a backend only when declared
- GIVEN a config that declares a backend and an otherwise identical config that does not
- WHEN each is serialized with `toIR`
- THEN the first IR contains the `backend` object verbatim and the second omits the field entirely.
