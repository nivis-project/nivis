# Spec delta: executor

## ADDED Requirements

### Requirement: Provider client reads datasources
The version-neutral provider `Client` interface SHALL provide `ReadDataSource`,
which sends a datasource type name and its (fully-known) config to the provider's
`ReadDataSource` and returns the read attributes (or error diagnostics). It SHALL
be plumbed through both the tfprotov6 and tfprotov5 backends, encoding the config
and decoding the result with the existing value codec. A datasource is never
planned, applied, or destroyed.

#### Scenario: a datasource type is read through the client
- GIVEN a configured provider and a datasource type with known config
- WHEN the executor calls ReadDataSource
- THEN the provider's ReadDataSource is invoked and the returned attributes are decoded to plain Go values.

#### Scenario: read works over both protocol versions
- GIVEN a v6 fake and a v5 fake each serving a datasource
- WHEN each is read
- THEN both return decoded attributes through the same Client interface.

### Requirement: Datasources read per phase in the fixpoint loop
The phase driver SHALL read each datasource **when its config inputs are fully
known**, using the same per-phase readiness determination that selects ready
resources, and SHALL place the datasource's returned attributes into the outputs
ledger keyed by its id. A datasource whose config is fully known reads in the
first phase; one whose config depends on a resource's apply-time output reads in a
later phase, after that output is in the ledger. A datasource SHALL be read at
most once per run (once its outputs are in the ledger it is done). Datasources
SHALL NOT be planned, applied, written to state, or destroyed. The fixpoint and
stuck-node detection SHALL account for datasources: a datasource whose inputs
never become known SHALL be reported as a stuck node with an actionable error
naming it, just like an unresolvable resource.

#### Scenario: a known-config datasource reads in the first phase
- GIVEN a datasource whose config has no unresolved refs
- WHEN the loop runs
- THEN it is read in the first phase and its outputs are in the ledger before dependent resources are applied.

#### Scenario: a datasource depending on a resource output reads later
- GIVEN a datasource whose config references a resource's apply-time output
- WHEN the loop runs
- THEN the resource applies in an earlier phase, the datasource reads in a later phase once that output is known, and a resource consuming the datasource applies after that.

#### Scenario: a datasource never reaches a known config is stuck
- GIVEN a datasource whose config references a value that never resolves
- WHEN the loop reaches a fixpoint with the datasource unread
- THEN the run fails with an error naming the datasource as stuck.

#### Scenario: datasources are not applied or destroyed
- GIVEN an IR with a datasource
- WHEN apply and destroy run
- THEN the datasource is read (on apply) but never planned, applied, written to state, or destroyed.
