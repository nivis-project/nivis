# Spec delta: executor

## ADDED Requirements

### Requirement: Outputs ledger
The executor SHALL maintain an outputs ledger in the contract format
`{ "phase": <n>, "outputs": { <id>: { <attr>: <value> } } }`, persisted to a
0600 file, supporting append of a resource's computed outputs and lookup of
whether `<id>.<attr>` is known. Sensitive outputs SHALL NOT be written to any
world-readable surface.

#### Scenario: append and lookup
- GIVEN an empty ledger
- WHEN resource A's outputs `{ value: "v" }` are appended
- THEN `A.value` is reported known and round-trips through save/load.

#### Scenario: ledger file is private
- WHEN the ledger is persisted
- THEN the file mode is 0600.

### Requirement: Phase driver loop
The executor SHALL drive resolution as a loop: evaluate Nix for the current
ledger, ingest the IR, select resources whose inputs are fully known (TF→TF
resolved and no derived leaves remaining), plan and apply them, append their
computed outputs to the ledger, and repeat while a phase resolved at least one
new output and unresolved work remains.

#### Scenario: a multi-phase chain resolves in order
- GIVEN A (ready), B (derived on A), C (derived on B)
- WHEN the driver runs
- THEN phase 1 applies A, phase 2 applies B (after re-eval resolves its input),
  phase 3 applies C — three apply phases total.

#### Scenario: only ready resources apply each phase
- GIVEN B depends on A via a derived value
- WHEN phase 1 runs
- THEN A is applied and B is not (its derived input is still unresolved).

### Requirement: Fixpoint and stuck-resource detection
The driver SHALL halt when a phase yields no new resolved value (fixpoint). If
unresolved refs or unapplied resources remain at halt, it SHALL return an
actionable error naming the stuck resources and the inputs they await.

#### Scenario: clean fixpoint
- WHEN the last resource applies and a further eval produces no new value
- THEN the driver halts reporting success with all resources applied.

#### Scenario: unresolvable dependency named
- GIVEN a resource whose input never becomes known (cycle or missing producer)
- WHEN the loop reaches fixpoint with it still pending
- THEN the error names that resource and the unresolved input.

### Requirement: TF→Nix feedback (the round trip)
The driver SHALL cause a Nix expression computed from a provider output (a
`__derived` value) to become concrete in a later phase's IR and be consumable by
a downstream resource or nixConsumer.

#### Scenario: derived value feeds a later resource
- GIVEN C.label = derived(B.endpoint) and B applied in an earlier phase
- WHEN Nix is re-evaluated with B.endpoint in the ledger
- THEN C.label is concrete in the new IR and C applies using it.

#### Scenario: consumer reads resolved outputs
- GIVEN a nixConsumer reading outputs from two providers
- WHEN the loop reaches fixpoint
- THEN the consumer's values are concrete (no remaining placeholders).
