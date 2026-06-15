# Spec delta: ir

## ADDED Requirements

### Requirement: Canonical IR shape
The system SHALL define a versioned JSON IR with top-level `schemaVersion`,
`providers`, `resources`, `edges`, and `nixConsumers`, as specified in
`docs/IR-CONTRACT.md`.

#### Scenario: well-formed IR validates
- GIVEN an IR with unique resource ids and every edge endpoint present
- WHEN it is validated by the Nix property test and Go `IngestIR`
- THEN both accept it without error.

#### Scenario: malformed IR is rejected with identity
- GIVEN an IR with an edge referencing a non-existent resource id
- WHEN `IngestIR` validates it
- THEN it fails with an error naming the offending edge and resource id.

### Requirement: Typed reference encoding
A not-yet-known cross-resource or cross-domain value SHALL be encoded as
`{ "__ref": { "resource": <id>, "path": [<key|index>...] } }`.

#### Scenario: nested list index ref
- GIVEN a config leaf referencing the first network's ip of resource R
- WHEN serialized
- THEN it is `{ "__ref": { "resource": "R", "path": ["net", 0, "ip"] } }`.

#### Scenario: ref into an expanded instance
- GIVEN a `for_each` over keys including "a", expanded in Nix
- WHEN another resource references that instance's attr
- THEN the ref's `resource` is the concrete id `<base>__a` (no special form).

### Requirement: Derived (Nix-computed) values
A config or consumer leaf computed by Nix from resource outputs SHALL be encoded
as `{ "__derived": { "inputs": [<id.attr>...] } }` and classified `*->Nix`,
resolvable only by re-evaluation, never in-executor.

#### Scenario: derived value forces a later phase
- GIVEN `name = "rec-" + A.value` where `A.value` is unknown
- WHEN phase 0 is evaluated
- THEN `name` is `{ "__derived": { "inputs": ["A.value"] } }`
- AND it becomes concrete only after `A` is applied and Nix is re-evaluated.

### Requirement: Unknown values toward providers
The executor SHALL present unresolved refs to providers as the tfprotov6 unknown
sentinel, never as `__ref`/`__derived` JSON.

#### Scenario: plan with unknown input
- GIVEN a resource whose `from` input is an unresolved `__ref`
- WHEN `PlanResourceChange` is called
- THEN `from` is sent as a tfprotov6 unknown value.

### Requirement: Expansion happens in Nix
`count`/`for_each` SHALL be expanded during Nix evaluation; the IR SHALL contain
only concrete instances and SHALL NOT contain `count`/`for_each` meta-arguments.

#### Scenario: no count in IR
- GIVEN a resource with `count = 3`
- WHEN serialized
- THEN the IR contains three concrete resources and no `count` field.

### Requirement: Sensitive values never enter the store
Sensitive outputs SHALL NOT appear in `nix eval` JSON output or any Nix store
path. They SHALL live only in the executor's 0600 outputs ledger and be injected
to re-evaluation via a private 0600 channel, referenced as `__sensitiveRef`.

#### Scenario: sensitive output not in eval output
- GIVEN a provider attribute marked sensitive that another resource consumes
- WHEN the IR is emitted by `nix eval`
- THEN the sensitive value is absent and represented only by a ref/placeholder.

### Requirement: Outputs-ledger injection format
The executor SHALL accumulate resolved outputs in a ledger
`{ "phase": <n>, "outputs": { <resource-id>: { <attr>: <value|__sensitiveRef> } } }`
injected into the flake `plan` argument on each phase.

#### Scenario: ledger drives re-eval
- GIVEN a phase-1 ledger containing `A.value`
- WHEN phase-2 Nix evaluation runs with the ledger injected
- THEN refs and `__derived` leaves depending on `A.value` resolve to concrete values.
