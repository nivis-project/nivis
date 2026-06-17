# Spec delta: ir

## MODIFIED Requirements

### Requirement: Outputs-ledger injection format
The executor SHALL accumulate resolved outputs in a ledger
`{ "phase": <n>, "outputs": { <resource-id>: { <attr>: <value|__sensitiveRef> } } }`
injected into the flake `plan` argument on each phase. The ledger MAY additionally
carry a `vars` object, `{ <name>: <value> }`, the resolved configuration
variables. `outputs` accumulates across phases; `vars` SHALL be constant across
all phases of a run (resolved once before phase 0 and re-injected unchanged each
phase). `vars` is OPTIONAL: a ledger with no variables MAY omit it, and a `plan`
that declares none MAY ignore it. A `vars` value SHALL be known data (string,
number, bool, or a JSON value for permissively-typed variables) and SHALL NOT be a
ref or unknown placeholder, since variables are known inputs, not
resolved-across-phases outputs.

#### Scenario: ledger drives re-eval
- GIVEN a phase-1 ledger containing `A.value`
- WHEN phase-2 Nix evaluation runs with the ledger injected
- THEN refs and `__derived` leaves depending on `A.value` resolve to concrete values.

#### Scenario: ledger carries phase, outputs, and optional vars
- GIVEN a phased run with one resolved output and one variable set
- WHEN the executor injects the ledger for the next phase
- THEN the injected JSON has the integer `phase`, the `outputs` map, and a `vars` map containing the variable's resolved value.

#### Scenario: vars is constant across phases
- GIVEN a run that takes three phases to reach a fixpoint, with variables set
- WHEN the ledger is injected on each phase
- THEN the `vars` object is identical on every phase, while `outputs` grows.

#### Scenario: vars is optional
- GIVEN a config that declares no variables and a run with none set
- WHEN the ledger is injected
- THEN `vars` MAY be absent, and the ledger remains valid.
