# Spec: ir

## Purpose
The canonical JSON IR is the frozen contract between the Nix library (Epic 1),
the provider schema codegen (Epic 2), and the Go executor (Epic 3/3.5). It is an
API: changing its shape requires an OpenSpec change against this capability first
and a `schemaVersion` bump. The authoritative prose lives in
`docs/IR-CONTRACT.md`; the machine-checkable form is `docs/ir-schema.json` plus
the conformance suite in `tests/ir-conformance/`. This spec records the requirements
both producers (`toIR`) and consumers (`IngestIR`) must satisfy.
## Requirements
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

### Requirement: Datasource nodes in the IR
The IR SHALL support a top-level `dataSources` array, distinct from `resources`,
for infrastructure the configuration **reads** rather than creates (the array is
optional and MAY be omitted when there are none). A datasource node SHALL have the
shape `{ "id", "provider", "type", "name", "config" }`, where:

- `id` is `data.<provider>.<type>.<name>`, unique across all datasource ids (and
  not colliding with a resource id);
- `provider` names a declared provider;
- `config` is an attribute tree whose leaves may be values, `__ref`, or
  `__derived`, exactly like a resource config.

A datasource node SHALL NOT carry `meta`/lifecycle: a datasource is read, never
planned, applied, or destroyed. A `__ref`/`__derived`/edge MAY target a datasource
id, and a datasource config MAY reference a resource or another datasource, so
datasources participate in the dependency graph and the phased fixpoint like any
other node. `dataSources` is OPTIONAL: an IR with none MAY omit it.

#### Scenario: a datasource node is carried distinctly from resources
- GIVEN a config declaring a datasource and a resource that references it
- WHEN toIR serializes the graph
- THEN the IR has a `dataSources` array containing `{ id: "data.<p>.<t>.<n>", provider, type, name, config }` and the resource's config carries a `__ref` to the datasource id.

#### Scenario: a datasource id is unique and namespaced
- GIVEN two datasources and a resource
- WHEN the IR is built
- THEN each datasource id begins `data.` and no two node ids (resource or datasource) collide.

#### Scenario: datasources are optional
- GIVEN a config with no datasources
- WHEN the IR is built
- THEN `dataSources` MAY be absent and the IR remains valid.

