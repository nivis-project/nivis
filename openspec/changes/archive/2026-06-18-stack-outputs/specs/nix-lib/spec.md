# Spec delta: nix-lib

## ADDED Requirements

### Requirement: Declare stack outputs with the outputs argument
`toIR` SHALL accept an optional `outputs` argument, an attrset mapping a name to a
value expression (which may contain refs/derived leaves over resource or
datasource outputs). Each named output SHALL be emitted as a reserved nixConsumer
with id `output.<name>` and value `{ value = <expr>; }`, so it is resolved by the
existing consumer machinery (fully concrete at fixpoint) without a new node type.
The `outputs` argument is optional; omitting it produces no output consumers.

#### Scenario: a declared output becomes a reserved consumer
- GIVEN `toIR { ...; outputs = { ip = r.refAttr "addr"; }; }`
- WHEN the IR is built
- THEN it contains a nixConsumer with id `output.ip` whose value carries the ref to `r.addr`.

#### Scenario: outputs may derive from multiple resources
- GIVEN an output whose value is a Nix-built string over two resources' outputs
- WHEN the IR is built and resolved against a ledger with both outputs
- THEN the `output.<name>` consumer resolves to the concrete computed value.

#### Scenario: outputs are optional
- GIVEN `toIR` called without `outputs`
- WHEN the IR is built
- THEN no `output.` consumers are present and the IR is otherwise unchanged.
