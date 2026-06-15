# Spec: executor

## Purpose
The Go executor is pure orchestration (no HCL, no policy): it ingests the frozen
IR, builds the dependency graph, resolves the references it can resolve itself
(TF→TF), and persists state. This capability records the behavior of that core —
everything the executor does that does not require contacting a provider. The
provider plugin client and the plan/apply engines build on this model; the
phased-eval loop (E3.5) drives it.
## Requirements
### Requirement: IR ingestion and validation
The executor SHALL read the JSON IR and validate it against the contract before
use, producing typed resource nodes, provider configs, meta-args, and a ref-edge
list. Malformed IR SHALL be rejected with an error naming the offending
resource, edge, or path.

#### Scenario: well-formed IR ingests
- WHEN `IngestIR` is given an IR with unique ids and every edge endpoint present
- THEN it returns a graph with one `ResourceNode` per resource and no error.

#### Scenario: dangling edge rejected with identity
- WHEN `IngestIR` is given an IR whose edge `to` names a non-existent resource id
- THEN it returns an error naming that edge and the missing resource id.

#### Scenario: duplicate id rejected
- WHEN `IngestIR` is given two resources sharing an id
- THEN it returns an error naming the duplicated id.

### Requirement: Reference classification
Each reference leaf SHALL be classified as TF→TF (appears in a
`resources[].config` as a `__ref`) or \*→Nix (appears in a `nixConsumers[].value`,
or is a `__derived` leaf anywhere), per the IR contract.

#### Scenario: config ref is TF→TF
- GIVEN a `__ref` leaf inside a resource's config
- WHEN the IR is ingested
- THEN that ref is classified TF→TF.

#### Scenario: derived leaf is star-to-Nix
- GIVEN a `__derived` leaf in a resource config or a consumer value
- WHEN the IR is ingested
- THEN that leaf is classified \*→Nix and is NOT resolvable in-executor.

### Requirement: DAG construction and cycle detection
The executor SHALL build a dependency graph from ref edges and `meta.dependsOn`,
detect cycles, and expose a deterministic ready-set traversal (resources whose
dependencies are all satisfied), honoring `depends_on`.

#### Scenario: topological readiness
- GIVEN A with no deps and B depending on A
- WHEN the ready set is computed with no outputs known
- THEN A is ready and B is not; after A's outputs are known, B becomes ready.

#### Scenario: explicit depends_on respected
- GIVEN B with `meta.dependsOn = [A]` and no value ref to A
- WHEN the ready set is computed
- THEN B is not ready until A is marked complete.

#### Scenario: cycle is detected and named
- GIVEN A referencing B and B referencing A
- WHEN the DAG is built
- THEN it returns a cycle error naming A and B.

### Requirement: TF→TF reference resolution in-executor
Given known resource outputs, the executor SHALL substitute resolved values into
dependent configs (TF→TF refs only) and report which resources are now fully
known versus still pending. \*→Nix leaves SHALL be left unresolved.

#### Scenario: ref resolves when target output known
- GIVEN B.config.from = `__ref`(A.value) and A.value known to be "x"
- WHEN resolution runs
- THEN B.config.from becomes "x" and B is reported fully known.

#### Scenario: derived leaf stays pending
- GIVEN C.config.label = `__derived`(B.endpoint) and B.endpoint known
- WHEN resolution runs
- THEN C.config.label remains a `__derived` leaf (resolved only by re-eval) and C is reported pending.

### Requirement: Lockable local JSON state backend
The executor SHALL persist resource states to a local JSON file with an advisory
lock, supporting get/set/list/delete by resource id. The format is internal (no
tfstate compatibility) and accessed behind an interface that admits future
remote backends.

#### Scenario: round-trip a resource state
- WHEN a resource state is set and the store reloaded from disk
- THEN get by id returns the same attributes.

#### Scenario: concurrent writers are serialized
- WHEN two state operations contend
- THEN the advisory lock serializes them and neither write is lost.

#### Scenario: partial state survives a crash mid-run
- GIVEN resource A's state has been set and persisted
- WHEN the process exits before B is applied
- THEN reopening the store still returns A's state.

### Requirement: Provider plugin client over go-plugin v6
The executor SHALL spawn a provider binary and complete the go-plugin/gRPC
protocol-6 handshake (matching the tfprotov6 server's magic cookie and protocol
version), obtaining a working `tfplugin6.ProviderClient`. Clients SHALL be pooled
by provider identity and closed on shutdown.

#### Scenario: spawn and handshake with a fake provider
- GIVEN the built `provider-alpha` binary
- WHEN the manager starts it and performs the handshake
- THEN `GetProviderSchema` over the returned client succeeds and lists `alpha_token`.

#### Scenario: pooled by identity
- WHEN two resources of the same provider are processed
- THEN the manager reuses one spawned process, not two.

### Requirement: Unknown values presented to the provider
The executor SHALL present any unresolved `__ref` attribute to the provider as
the tfprotov6 unknown value, never as the `__ref` JSON, when planning a resource
whose config still contains that unresolved reference.

#### Scenario: unresolved ref becomes unknown at plan
- GIVEN a resource whose `from` is an unresolved `__ref`
- WHEN the plan engine encodes config for `PlanResourceChange`
- THEN `from` is sent as the protocol unknown value.

### Requirement: Plan engine
The plan engine SHALL, for a ready resource, fetch the provider schema, encode
config (resolved values known; unresolved refs unknown), call
`PlanResourceChange`, and produce a human-readable plan without side effects.

#### Scenario: plan reports computed attrs as unknown
- GIVEN a ready `alpha_token` with `label` known
- WHEN it is planned
- THEN the plan shows `id` and `value` as known-after-apply (unknown now).

### Requirement: Apply engine writes computed outputs to state
The apply engine SHALL call `ApplyResourceChange`, extract the now-known computed
outputs, and persist them to the state store. State SHALL be written after each
successful apply so a failure mid-run leaves prior successes recorded.

#### Scenario: apply yields and persists deterministic outputs
- GIVEN a planned `alpha_token` with `label = "rec-X"` (counter 0)
- WHEN it is applied
- THEN state for that resource records `id = "alpha-0"` and `value = "alpha:rec-X:0"`.

#### Scenario: partial state persists on mid-run failure
- GIVEN A applies successfully and B then fails
- WHEN the run aborts
- THEN A's computed outputs remain in the state store.

### Requirement: Single-provider plan/apply integration against fakes
The change SHALL include an integration test that drives the real fake provider
binaries end-to-end (spawn, handshake, plan, apply) with no network, asserting
the persisted outputs equal the fakes' deterministic derivations.

#### Scenario: alpha end-to-end through the manager
- GIVEN an IR with one `alpha_token` (label "rec-X") and the alpha binary
- WHEN the executor plans and applies it through the plugin manager
- THEN the state store holds `id = "alpha-0"`, `value = "alpha:rec-X:0"`.

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

### Requirement: Destroy in reverse dependency order
The executor SHALL destroy resources in reverse dependency order (a dependent
before the resources it depends on), calling the provider to delete each and
removing it from the state store. It SHALL refuse to destroy a resource marked
`lifecycle.preventDestroy` with an actionable error naming it.

#### Scenario: reverse-order teardown
- GIVEN applied resources A, then B (depends on A), then C (depends on B)
- WHEN destroy runs
- THEN the provider is asked to delete C, then B, then A, in that order
- AND each is removed from the state store.

#### Scenario: preventDestroy is honored
- GIVEN a resource with `lifecycle.preventDestroy = true`
- WHEN destroy targets it
- THEN it fails with an error naming the resource and the resource remains in state.

### Requirement: Refresh reconciles via ReadResource without planning
The executor SHALL refresh by calling `ReadResource` for each resource in state
with its stored state and writing back the reconciled result. Refresh SHALL NOT
plan or apply changes.

#### Scenario: refresh leaves a converged state unchanged
- GIVEN state for resources whose providers return the same values on read
- WHEN refresh runs
- THEN each resource's state is unchanged and no apply is performed.

### Requirement: Command-line interface
The system SHALL provide a `nixform` CLI with `plan`, `apply`, `destroy`,
`refresh`, and `state` (`list`, `show`, `rm`) subcommands, a `--target` id
filter, and `--state`/`--flake` options. `plan`/`apply` drive the phased-eval
loop; `destroy`/`refresh` use their engines.

#### Scenario: state list shows applied resources
- GIVEN a state store with applied resources
- WHEN `nixform state list` runs
- THEN it prints each resource id.

#### Scenario: state rm removes one resource
- GIVEN a state store containing resource R
- WHEN `nixform state rm R` runs
- THEN R is no longer in the store.

### Requirement: Version-neutral provider client interface
The executor SHALL access providers through a version-neutral `provider.Client`
interface exposing `GetSchema`, `Plan`, `Apply`, and `Read`, exchanging
normalized Go types (schema model, attribute maps, diagnostics) rather than
protocol-version-specific protobuf. Plan/apply/destroy/refresh/codegen SHALL
depend only on this interface.

#### Scenario: v6 backend satisfies the interface
- GIVEN the tfprotov6 backend
- WHEN the executor drives a fake v6 provider through GetSchema/Plan/Apply/Read
- THEN it works through the `provider.Client` interface with no protocol types leaking to callers.

#### Scenario: behavior is unchanged by the abstraction
- GIVEN the existing fake-provider e2e and unit suites
- WHEN they run against the refactored executor
- THEN they pass unchanged (the abstraction introduces no behavior change).

### Requirement: Manager returns a version-neutral client
The plugin manager SHALL return a `provider.Client` for a spawned provider,
selecting the protocol backend internally; callers SHALL NOT depend on the
negotiated protocol version.

#### Scenario: manager hands back a Client
- GIVEN a provider binary
- WHEN the manager spawns it
- THEN it returns a `provider.Client` the executor can use directly.

### Requirement: tfprotov5 provider backend
The executor SHALL support tfprotov5 providers via a backend implementing the
version-neutral `provider.Client` interface, covering schema listing/fetch,
plan, apply, read, and destroy with a v5 value codec.

#### Scenario: drive a v5 provider through the neutral interface
- GIVEN a fake tfprotov5 provider
- WHEN the executor runs GetSchema/Plan/Apply/Read/Destroy via provider.Client
- THEN each works and computed outputs become known at apply, identical to v6.

### Requirement: Protocol negotiation per provider
The plugin manager SHALL offer both protocol 5 and protocol 6 plugin sets and
SHALL build the backend matching the version go-plugin negotiates with the spawned
provider. Callers SHALL NOT specify or depend on the protocol version.

#### Scenario: v5 provider negotiates v5
- GIVEN a provider binary that serves only protocol 5
- WHEN the manager spawns it
- THEN the handshake negotiates v5 and a v5-backed provider.Client is returned.

#### Scenario: v6 provider still negotiates v6
- GIVEN a provider that serves protocol 6 (the existing fakes)
- WHEN the manager spawns it
- THEN it negotiates v6 and behavior is unchanged.

### Requirement: v5 providers work end-to-end in the phased loop
A tfprotov5 provider SHALL participate in the phased-eval loop exactly as a v6
provider: its computed outputs feed the ledger and unlock dependent resources.

#### Scenario: a v5 resource resolves in the loop
- GIVEN a graph mixing a v5 provider's resource with a dependent resource
- WHEN the driver runs to fixpoint
- THEN the v5 resource applies and its outputs resolve the dependent.

