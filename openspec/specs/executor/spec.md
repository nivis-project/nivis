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
The plan engine SHALL, for a ready resource, fetch the provider schema, look up
the resource's **prior state** (its stored attributes, or none if new), encode
config (resolved values known; unresolved refs unknown), call
`PlanResourceChange` with that prior state, and produce a human-readable plan
without side effects. The plan SHALL surface whether the change is a create, an
in-place update, a replacement (the provider's `RequiresReplace`), or a **no-op**
(the planned state equals the prior state with nothing unknown and no replace),
and render them distinctly.

#### Scenario: plan reports computed attrs as unknown
- GIVEN a ready `alpha_token` with `label` known
- WHEN it is planned
- THEN the plan shows `id` and `value` as known-after-apply (unknown now).

#### Scenario: plan distinguishes create, update, replace, and no-op
- GIVEN one resource not in state, one in state with a changed normal attribute, one in state with a changed force-new attribute, and one in state whose config is unchanged
- WHEN they are planned
- THEN the plan marks them as create, update, replace, and no-op respectively.

### Requirement: Apply engine writes computed outputs to state
The apply engine SHALL call `ApplyResourceChange` with the planned state and the
resource's **prior state** (null for a create), extract the now-known computed
outputs, and persist them to the state store. For a replacement it SHALL destroy
the prior resource before creating the new one so none is orphaned. When the plan
is a **no-op** (planned equals prior), the apply engine SHALL NOT call
`ApplyResourceChange` (nor destroy); it SHALL leave the prior state in place and
use the prior attributes as that resource's outputs, so dependents still resolve.
State SHALL be written after each successful apply so a failure mid-run leaves
prior successes recorded.

#### Scenario: apply yields and persists deterministic outputs
- GIVEN a planned `alpha_token` with `label = "rec-X"` (counter 0)
- WHEN it is applied
- THEN state for that resource records `id = "alpha-0"` and `value = "alpha:rec-X:0"`.

#### Scenario: an unchanged resource is a no-op on re-apply
- GIVEN a resource already in state whose config is unchanged
- WHEN it is applied again
- THEN the provider's ApplyResourceChange is not called for it, its state is unchanged, and its prior outputs are still available to dependents.

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
The system SHALL provide a `Nivis` CLI with `plan`, `apply`, `destroy`,
`refresh`, and `state` (`list`, `show`, `rm`) subcommands, a `--target` id
filter, and `--state`/`--flake` options. `plan`/`apply` drive the phased-eval
loop; `destroy`/`refresh` use their engines.

#### Scenario: state list shows applied resources
- GIVEN a state store with applied resources
- WHEN `Nivis state list` runs
- THEN it prints each resource id.

#### Scenario: state rm removes one resource
- GIVEN a state store containing resource R
- WHEN `Nivis state rm R` runs
- THEN R is no longer in the store.

### Requirement: Version-neutral provider client interface
The executor SHALL access providers through a version-neutral `provider.Client`
interface exposing `GetSchema`, `Plan`, `Apply`, and `Read`, exchanging
normalized Go types (schema model, attribute maps, diagnostics) rather than
protocol-version-specific protobuf. The plan request SHALL carry an optional
prior state and the plan result SHALL carry whether replacement is required; the
apply request SHALL carry the prior state. Plan/apply/destroy/refresh/codegen
SHALL depend only on this interface.

#### Scenario: v6 backend satisfies the interface
- GIVEN the tfprotov6 backend
- WHEN the executor drives a fake v6 provider through GetSchema/Plan/Apply/Read
- THEN it works through the `provider.Client` interface with no protocol types leaking to callers.

#### Scenario: prior state and replace flow through the interface
- GIVEN a resource with prior state and a force-new change
- WHEN it is planned and applied through `provider.Client`
- THEN the backend sends the prior state to the provider and reports `RequiresReplace`, with no protocol types leaking to callers.

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

### Requirement: Provider schema is fetched once per spawned provider
A provider backend SHALL fetch the full provider schema at most once per spawned
process and serve `GetSchema` and `ListResourceTypes` from that cached response,
so operations over many resource types do not issue O(resources) schema RPCs.

#### Scenario: many GetSchema calls, one RPC
- GIVEN a spawned provider with many resource types
- WHEN GetSchema is called for each type and ListResourceTypes is called
- THEN the provider's GetProviderSchema RPC is invoked at most once.

### Requirement: Provider logs do not flood output
The plugin manager SHALL configure spawned providers with a quiet logger so their
debug/trace logging is not written to the executor's stderr; errors and operation
results remain visible.

#### Scenario: a chatty provider does not flood stderr
- GIVEN a provider that emits trace-level logs during schema fetch
- WHEN it is spawned and queried
- THEN its trace/debug logs are suppressed from the executor's stderr.

### Requirement: Value codec encodes and decodes collections and objects
The value codec SHALL encode and decode `list`, `set`, `tuple`, `map`, and
`object` (nested) attribute types, in addition to the scalar string/number/bool
types, for both the tfprotov5 and tfprotov6 backends. Encoding maps a Go
`[]interface{}` to list/set/tuple values and a `map[string]interface{}` to
map/object values, recursing into element/attribute types; decoding is the
symmetric inverse.

#### Scenario: round-trip a list of strings
- GIVEN a `list(string)` attribute with value `["a","b"]`
- WHEN it is encoded to a DynamicValue and decoded back
- THEN the decoded Go value is `["a","b"]`.

#### Scenario: round-trip a map of strings
- GIVEN a `map(string)` attribute with value `{ env = "prod" }`
- WHEN encoded and decoded
- THEN the decoded value is `{ "env": "prod" }`.

#### Scenario: round-trip a nested object
- GIVEN an `object({ name = string, ports = list(number) })` value
  `{ name = "x", ports = [80, 443] }`
- WHEN encoded and decoded
- THEN the decoded value preserves the nested structure and element types.

#### Scenario: unknown collection attribute at plan
- GIVEN a computed `list(string)` attribute with no config value
- WHEN config is encoded for plan
- THEN that attribute is the tftypes unknown value (unchanged from scalar behavior).

#### Scenario: unsupported type errors clearly
- GIVEN an attribute of a type the codec does not support (e.g. DynamicPseudoType)
- WHEN encoding is attempted
- THEN it returns an error naming the unsupported type rather than panicking.

### Requirement: Providers are configured before plan/apply
The executor SHALL call the provider's configure RPC (ConfigureProvider for v6,
Configure for v5) once per spawned provider, before any plan/apply/read, passing
the provider's `config` from the IR encoded against the provider config schema.
Attributes absent from the IR config SHALL be sent as null so the provider can
apply its own defaults (e.g. the AWS SDK credential/region chain). Configure
diagnostics SHALL surface as an error.

#### Scenario: configure happens before plan
- GIVEN a provider that requires configuration before planning
- WHEN the manager returns a client and a plan is requested
- THEN Configure has been called once for that provider first.

#### Scenario: configure errors surface
- GIVEN a provider that returns an error diagnostic from configure
- WHEN configuration runs
- THEN the operation fails with an error containing the diagnostic.

### Requirement: Schema object types include nested blocks
The executor SHALL include nested blocks (`block_types`) when building the
tftypes object type for a provider config or resource schema, as attributes whose
type reflects the nesting mode — SINGLE/GROUP as an object, LIST as a
list(object), SET as a set(object), MAP as a map(object) — recursing into the
nested block's own attributes and blocks. Omitting them yields a non-conforming
value (e.g. AWS configure fails "an object with 35 attributes is required").

#### Scenario: provider config object includes its blocks
- GIVEN a provider whose config has flat attributes plus nested blocks
- WHEN the provider config object type is built
- THEN it includes an attribute per nested block of the correct
  collection-of-object type, so a value with those attributes (null) conforms and
  Configure succeeds.

### Requirement: Real AWS provider can be planned read-only
The executor SHALL be able to configure the real AWS provider (credentials and
region resolved from the environment) and plan a resource without required inputs
(`aws_s3_bucket`), producing a planned state with no error and creating no
resource.

#### Scenario: plan aws_s3_bucket against real AWS
- GIVEN the real AWS provider and valid credentials in the environment
- WHEN the provider is configured and `aws_s3_bucket` is planned with no inputs
- THEN a planned state is returned, no error occurs, and no resource is created.

### Requirement: Create, update, and replace from prior state
The executor SHALL decide a resource's operation from its prior state and the
provider's plan, generically for every resource type (no per-resource code): when
there is **no** prior state for the resource id, it SHALL create; when prior state
exists and the plan does **not** require replacement, it SHALL update the resource
in place; when prior state exists and the plan **requires replacement** (a
force-new attribute changed), it SHALL replace it by destroying the prior resource
and then creating the new one, leaving no orphaned resource. The decision SHALL
derive from `(prior state present?, RequiresReplace?)` returned by the provider,
which itself judges create/update/replace from `(PriorState, ProposedNewState)`
and its schema.

#### Scenario: changed normal attribute updates in place
- GIVEN a resource already in state whose config changes only a non-force-new attribute
- WHEN it is applied again
- THEN the provider receives the stored prior state, the resource is updated in place (not recreated), and state reflects the new attributes.

#### Scenario: changed force-new attribute replaces
- GIVEN a resource already in state whose config changes a force-new attribute
- WHEN it is applied again
- THEN the executor destroys the prior resource and creates the new one, exactly one resource remains, and no prior resource is orphaned.

#### Scenario: a new resource is still created
- GIVEN a resource id with no prior state
- WHEN it is applied
- THEN it is created (prior state null), as before.

### Requirement: Refresh prior state before planning
Before planning a resource that is present in state, the executor SHALL by
default **refresh** its prior state by reading the resource through the provider
(`ReadResource`) and use the read result as the prior state, so the plan reflects
the real world rather than the stored record. The behavior SHALL be:
- the read returns attributes → those are the prior state (so out-of-band drift
  is planned against, and an unchanged resource is still a no-op);
- the read returns **empty** (the resource was deleted out-of-band) → the
  resource is treated as having no prior state and is planned/applied as a
  **create**;
- a resource not in state → unchanged (a create; not read).
The executor SHALL provide an opt-out (a `--refresh=false` flag, default true) to
plan against stored state without reading the provider. On apply, a refreshed
resource's state SHALL be persisted.

#### Scenario: drift is planned against the real state
- GIVEN a resource in state whose real attributes have drifted out-of-band
- WHEN it is planned with refresh on
- THEN the provider is read and the plan is computed against the read (drifted) state, not the stale stored state.

#### Scenario: an out-of-band deletion re-creates
- GIVEN a resource in state that was deleted outside Nivis (the provider read returns empty)
- WHEN it is planned/applied with refresh on
- THEN it is planned as a create and apply re-creates it.

#### Scenario: refresh can be disabled
- GIVEN a resource in state
- WHEN plan/apply runs with `--refresh=false`
- THEN the provider is NOT read and the stored state is used as the prior state.

### Requirement: The executor realises __build leaves before apply
Before applying a resource, the executor SHALL walk that resource's resolved
config for `__build` leaves and **realise** each one's store path that is not
already valid (building the derivation, e.g. via `nix-store --realise` on the
store root), then substitute the concrete path into the config sent to the
provider. This happens **per resource, as it becomes ready**, so across the phased
loop only the builds reachable from the resources ready in a given phase are
performed — never "everything" — and a build whose inputs come from an earlier
resource's outputs (a Nix expression that consumes a prior apply's value) is
realised in the later phase once it is evaluable. An author opt-out (`--no-build`)
SHALL skip realising (assuming paths are already built). A realise failure SHALL
produce an error naming the store path.

#### Scenario: a build output is realised before the provider reads it
- GIVEN a resource whose config has a `__build` leaf for an unbuilt store path
- WHEN that resource is applied
- THEN the executor realises the path first, and the provider receives the concrete (now-existing) path — no "no such file or directory".

#### Scenario: only what is ready this phase is built
- GIVEN resource B's `__build` derivation depends on resource A's apply-time output
- WHEN the loop runs
- THEN A is applied first, the config re-evaluates so B's derivation becomes known, and B's build is realised in the later phase (not before A) — the build participates in the fixpoint.

#### Scenario: --no-build skips realising
- GIVEN `--no-build` and a resource with a `__build` leaf
- WHEN it is applied
- THEN the executor does not build; it uses the path as-is (and the provider errors if it is missing — the user opted out).

### Requirement: Actionable error for nested-block list-vs-single mistakes
When converting a decoded config value to a provider value, the executor SHALL
emit an actionable error, naming the fix, for the two common nested-block shape
mistakes, instead of a type-jargon message. The offending attribute name SHALL be
included (the codec already prefixes per-key errors).

- When the target type is a **list, set, or tuple** and the supplied value is an
  **attrset** (decoded as a map), the error SHALL state that the block is
  list-nested and instruct the user to wrap the attrset in a one-element list
  (`[ { ... } ]`), rather than reporting `expected array for tftypes.List[...],
  got map`.
- When the target type is a **single-nested object or map** and the supplied
  value is a **list**, the error SHALL state that the block takes a single attrset
  and instruct the user to pass `{ ... }` rather than a list.

Valid inputs SHALL be unaffected, and other scalar/type mismatches SHALL keep
their existing messages. This requirement is about the error text only; it
changes no successful-conversion behavior and does not alter the IR.

#### Scenario: list-nested block given a bare attrset
- GIVEN a resource config where a list-nested block (e.g. `disk_container`, typed `List[Object{...}]`) is written as a bare attrset `{ ... }`
- WHEN the executor codes the config to a provider value
- THEN it returns an error that names the attribute and instructs the user to wrap the value in a one-element list `[ { ... } ]`
- AND the error does not consist solely of `expected array for tftypes.List[...], got map`.

#### Scenario: single-nested block given a list
- GIVEN a resource config where a single-nested block (typed `Object{...}`) is written as a list `[ { ... } ]`
- WHEN the executor codes the config to a provider value
- THEN it returns an error that names the attribute and instructs the user to pass a single attrset `{ ... }`, not a list.

#### Scenario: valid nested blocks still code successfully
- GIVEN a config where every nested block uses the correct shape (a one-element list for a list-nested block, a single attrset for a single-nested block)
- WHEN the executor codes the config to a provider value
- THEN conversion succeeds with no error and produces the same value as before this change.

