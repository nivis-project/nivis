# Spec delta: executor

## ADDED Requirements

### Requirement: Resolve declared stack outputs after apply
The executor SHALL resolve a run's declared outputs (the reserved `output.<name>`
nixConsumers) to concrete values, by seeding the ledger from current state and
re-evaluating read-only (the same seed-from-state approach `plan` uses), then
collecting the `output.<name>` consumers from the resulting IR and unwrapping each
`{ value }` to its resolved value. The result SHALL be a map of output name to
value. An output referencing a resource not yet in state SHALL resolve once that
resource is applied; outputs of a fully-applied stack SHALL all be concrete.
Sensitive output values SHALL be handled by the existing sensitive-value rules and
SHALL NOT be written world-readable.

#### Scenario: outputs resolve against current state
- GIVEN an applied stack that declared `output.url` derived from a resource's attribute
- WHEN outputs are resolved
- THEN the result maps `url` to the concrete value computed from the applied resource.

#### Scenario: outputs spanning multiple resources are concrete after apply
- GIVEN outputs derived from two resources applied across phases
- WHEN outputs are resolved after a full apply
- THEN every declared output is a concrete value (no placeholders).
