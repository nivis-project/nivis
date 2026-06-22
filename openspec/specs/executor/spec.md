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
remote backends. The interface SHALL additionally provide whole-document access:
`Snapshot` returns the entire state as bytes (the canonical state document) and
`Restore` replaces the entire state from bytes (parsing and validating it as a
state document, then writing atomically under the lock). These two operations are
the document-level seam a remote backend implements, so whole-state read/replace
works identically across backends.

Acquiring the advisory lock SHALL NOT block indefinitely: it SHALL time out after
a bounded interval and, on failure, return an actionable error stating that the
state appears locked by another process and naming the lock file, rather than
hanging.

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

#### Scenario: snapshot round-trips through restore
- GIVEN a store with several resource states
- WHEN its `Snapshot` bytes are passed to `Restore` on an empty store
- THEN that store then lists exactly the same resource states.

#### Scenario: restore replaces the whole document
- GIVEN a store containing resource X
- WHEN `Restore` is called with a snapshot that contains only resource Y
- THEN the store afterwards contains Y and not X.

#### Scenario: restore rejects a malformed document
- WHEN `Restore` is called with bytes that are not a valid state document
- THEN it returns an error and the existing state is left unchanged.

#### Scenario: a contended lock fails with an actionable error, not a hang
- GIVEN another process holds the state lock
- WHEN a state operation cannot acquire the lock within the timeout
- THEN it returns an error naming the lock file and saying the state appears locked, rather than blocking forever.

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

When planning an already-applied stack, the plan SHALL read each side-effect-free
**datasource** whose inputs are known and seed its outputs into the resolution
ledger before classifying resources, using the same readiness determination the
apply loop uses. A resource whose config is fully resolvable only because it reads
a datasource SHALL therefore be planned against its provider and reported with its
true operation (a no-op when its config is unchanged), NOT reported as an update
merely because a datasource it depends on was not read. Datasource reads during
plan SHALL remain side-effect free: datasources are never planned, applied, or
written to state.

#### Scenario: plan reports computed attrs as unknown
- GIVEN a ready `alpha_token` with `label` known
- WHEN it is planned
- THEN the plan shows `id` and `value` as known-after-apply (unknown now).

#### Scenario: plan distinguishes create, update, replace, and no-op
- GIVEN one resource not in state, one in state with a changed normal attribute, one in state with a changed force-new attribute, and one in state whose config is unchanged
- WHEN they are planned
- THEN the plan marks them as create, update, replace, and no-op respectively.

#### Scenario: a datasource-dependent resource plans as no-op when unchanged
- GIVEN an applied stack with a datasource feeding a resource's config, re-planned with the config unchanged
- WHEN plan runs
- THEN the datasource is read into the ledger, the dependent resource is planned against its provider and reported as a no-op, not as an update.

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
SHALL depend only on this interface. The normalized schema returned by
`GetSchema` SHALL include the resource's **nested blocks**: for each, a name, a
nesting mode (single, list, set, or map), and its inner attributes (recursively),
so callers (notably codegen) need not parse protocol-specific protobuf to learn a
resource's blocks. Both the v6 and v5 backends SHALL populate this from the
provider schema's block types.

#### Scenario: v6 backend satisfies the interface
- GIVEN a v6 provider
- WHEN accessed through the manager
- THEN it is usable via the version-neutral client for schema/plan/apply/read.

#### Scenario: the normalized schema surfaces nested blocks
- GIVEN a resource whose provider schema declares a list-nested block and a single-nested block
- WHEN its schema is fetched through the version-neutral client (v6 or v5)
- THEN the returned schema reports both blocks with their nesting modes and inner attributes.

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

**Schema fetching SHALL NOT configure the provider.** Codegen needs only
`GetProviderSchema` (via `ListResourceTypes`/`GetSchema`), which the protocol
allows before configuration, so the executor SHALL provide a configure-free way to
obtain a client for schema fetching (distinct from the plan/apply client, which
still configures). This SHALL NOT change the plan/apply/refresh/destroy path: those
SHALL still configure the provider. The configure-free path SHALL therefore work
against providers that reject an unconfigured/all-null configure (credential-
requiring providers such as proxmox, azurerm, google), whose schema is otherwise
unreachable to codegen.

#### Scenario: configure happens before plan
- GIVEN a provider that requires configuration before planning
- WHEN the manager returns a client and a plan is requested
- THEN Configure has been called once for that provider first.

#### Scenario: configure errors surface
- GIVEN a provider that returns an error diagnostic from configure
- WHEN configuration runs
- THEN the operation fails with an error containing the diagnostic.

#### Scenario: schema fetch skips configure
- GIVEN a provider whose configure rejects an all-null config (credential-requiring)
- WHEN a client is obtained via the configure-free schema path and its schema is fetched
- THEN Configure is not called and the schema is returned successfully.

#### Scenario: plan/apply still configures that same provider
- GIVEN the same configure-rejecting provider
- WHEN a plan/apply client is obtained (the normal path)
- THEN Configure is still called and the operation fails with the configure diagnostic (the plan/apply contract is unchanged).

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

### Requirement: Resolve and inject variables with Terraform precedence
The executor SHALL resolve a variable map before evaluation and inject it as the
ledger `vars` object on every phase. Values SHALL be resolved from these sources,
**lowest to highest priority**, so a later source overrides an earlier one for the
same name:

1. environment variables named `NIVIS_VAR_<name>`;
2. `--var-file <path>` (a JSON object of name to value); when given multiple
   times, a later file overrides an earlier one;
3. `--var name=value` flags; when given multiple times, a later flag overrides an
   earlier one.

Declared defaults are NOT applied by the executor; they are the Nix layer's job
(`mkVars` fills an unset variable from its default). The executor only collects
the externally-supplied values above. The resolved `vars` map SHALL be the same
for every phase of the run. A malformed `--var` (no `=`) or an unreadable or
non-object `--var-file` SHALL produce an actionable error that names the offending
input. Variable values SHALL travel only in the 0600 ledger file, never on the Nix
command line, preserving the existing purity and secret-handling guarantees.

#### Scenario: an explicit flag overrides env and file
- GIVEN `NIVIS_VAR_region=eu-west-1`, a var-file setting `region=eu-central-1`, and `--var region=us-east-1`
- WHEN the executor resolves variables
- THEN `vars.region` is `us-east-1` (the flag wins).

#### Scenario: a var-file overrides the environment
- GIVEN `NIVIS_VAR_region=eu-west-1` and a var-file setting `region=eu-central-1`, with no `--var region`
- WHEN the executor resolves variables
- THEN `vars.region` is `eu-central-1` (the file beats env).

#### Scenario: later flags and files override earlier ones
- GIVEN `--var x=1 --var x=2` (and, separately, two `--var-file`s both setting `y`)
- WHEN the executor resolves variables
- THEN `x` is `2`, and `y` is the value from the later file.

#### Scenario: a malformed input is an actionable error
- GIVEN `--var notanassignment` (no `=`) or a `--var-file` that is not a readable JSON object
- WHEN the executor resolves variables
- THEN it fails with an error naming the offending flag or file, not a stack trace.

#### Scenario: variables are injected on every phase
- GIVEN a run that takes more than one phase and has variables set
- WHEN each phase is evaluated
- THEN the same resolved `vars` map is present in the injected ledger every phase.

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

### Requirement: Apply result groups applied nodes by phase
The phase driver's apply result SHALL report, in addition to the flat ordered list
of applied ids and the phase count, the ids applied in **each** phase, in phase
order, and for each applied id whether it was a **resource** (planned/applied) or
a **datasource** (read), and for each applied **resource** the **operation** it
resolved as (create, in-place update, replace, or no-op). This is reporting
metadata only: it SHALL NOT change which nodes are applied, the order, or any
apply behaviour. It exists so a caller can render the fixpoint (which nodes
resolved in which phase), distinguish a read from a create, and report the true
change type rather than assuming every applied node is a create.

#### Scenario: per-phase grouping reflects the fixpoint
- GIVEN a run that applies resources across three phases (a Nix-mediated chain)
- WHEN it completes
- THEN the result reports three phase groups, each listing the ids applied in that phase, and their concatenation in phase order equals the flat applied list.

#### Scenario: a datasource read is marked as a read
- GIVEN a run that reads a datasource and applies a resource
- WHEN it completes
- THEN the result marks the datasource id as a read and the resource id as an applied resource.

#### Scenario: an applied resource carries its operation
- GIVEN a first run that creates a resource and a second run of the same unchanged config
- WHEN each completes
- THEN the first run reports the resource's operation as create, and the second run reports it as no-op (not create), with its stored id and computed values unchanged between the two runs.

### Requirement: Resolve declared stack outputs after apply
The executor SHALL resolve a run's declared outputs (the reserved `output.<name>`
nixConsumers) to concrete values, by seeding the ledger from current state and
re-evaluating read-only (the same seed-from-state approach `plan` uses), then
collecting the `output.<name>` consumers from the resulting IR and unwrapping each
`{ value }` to its resolved value. The result SHALL be a map of output name to
value. An output referencing a resource not yet in state SHALL resolve once that
resource is applied; outputs of a fully-applied stack SHALL all be concrete.
Because datasources are read, not persisted to state, output resolution SHALL
**re-read the ready datasources** (reads are pure and idempotent), add their
results to the ledger, and re-evaluate, so an output (or consumer) that references
a datasource result resolves to a concrete value rather than remaining an
unresolved reference. Sensitive output values SHALL be handled by the existing
sensitive-value rules and SHALL NOT be written world-readable.

#### Scenario: outputs resolve against current state
- GIVEN an applied stack that declared `output.url` derived from a resource's attribute
- WHEN outputs are resolved
- THEN the result maps `url` to the concrete value computed from the applied resource.

#### Scenario: outputs spanning multiple resources are concrete after apply
- GIVEN outputs derived from two resources applied across phases
- WHEN outputs are resolved after a full apply
- THEN every declared output is a concrete value (no placeholders).

#### Scenario: an output referencing a datasource resolves
- GIVEN an applied stack with an output equal to a datasource's result (the datasource is not in state)
- WHEN outputs are resolved standalone (e.g. `nivis output`)
- THEN the datasource is re-read and the output is the concrete result, not a `__ref`.

### Requirement: S3 remote state backend
The executor SHALL provide an S3-backed implementation of the `Store` interface,
selected when the IR declares `backend.type == "s3"`. It SHALL store the entire
state **document** (the same canonical JSON `Snapshot`/`Restore` bytes the local
store uses, Nivis's own format, no tfstate compatibility) as a single S3 object at
the configured `bucket`/`key`, and SHALL implement per-resource `Get`/`Set`/
`Delete`/`List` as read-modify-write of that object. Every write SHALL request
server-side encryption. Credentials SHALL come from the AWS default credential
chain (environment, shared profile, instance role), NEVER from the IR `backend`;
only the location (`bucket`, `key`, `region`, and an optional `endpoint` override)
comes from `backend`. A missing object SHALL read as an empty state document (a
fresh stack), not an error.

#### Scenario: round-trip a resource state through S3
- GIVEN an s3-backed store
- WHEN a resource state is set and the store is reopened against the same bucket/key
- THEN get by id returns the same attributes (the document was persisted to the S3 object).

#### Scenario: a fresh (missing) object reads as empty
- GIVEN an s3-backed store whose object does not yet exist
- WHEN the state is listed
- THEN it returns empty with no error, and the first write creates the object.

#### Scenario: writes are server-side encrypted
- GIVEN an s3-backed store
- WHEN it writes the state object
- THEN the PutObject request specifies server-side encryption.

#### Scenario: snapshot/restore round-trip through S3
- GIVEN an s3-backed store with several resource states
- WHEN its `Snapshot` bytes are passed to `Restore` on a store over a different key
- THEN that store then lists exactly the same resource states.

#### Scenario: credentials are not taken from config
- GIVEN an IR `backend` for s3
- WHEN the store is opened
- THEN the bucket/key/region come from `backend` and the credentials come from the AWS default chain, and no credential field in `backend` is read.

### Requirement: State backend selection from the IR
The executor SHALL choose the state backend from the IR's optional `backend`
block: `type == "s3"` selects the S3 store; an absent `backend` (or a local type)
uses the local file store (today's default and the `--state` path). Selection SHALL
validate the backend's required keys (for s3: `bucket`, `key`, `region`) and fail
with an actionable error when a required key is missing or the `type` is
unsupported. All commands that use state (plan, apply, destroy, refresh, the state
subcommands) SHALL operate through the selected `Store` without other changes,
since they depend only on the interface.

#### Scenario: s3 backend selects the S3 store
- GIVEN an IR declaring `backend = { type = "s3"; bucket; key; region }`
- WHEN a state-using command runs
- THEN it operates against the S3 store, not the local file.

#### Scenario: no backend uses the local file store
- GIVEN an IR with no `backend`
- WHEN a state-using command runs
- THEN it uses the local file store at the `--state` path (unchanged behaviour).

#### Scenario: an unsupported backend type errors clearly
- GIVEN an IR whose `backend.type` is not a supported backend
- WHEN the store is opened
- THEN it fails with an actionable error naming the unsupported type.

#### Scenario: a missing required s3 key errors clearly
- GIVEN an s3 `backend` missing `bucket` (or `key`/`region`)
- WHEN the store is opened
- THEN it fails with an actionable error naming the missing key.

### Requirement: Advisory state locking for the S3 backend
The S3 backend SHALL provide an advisory lock so two concurrent mutating runs
cannot corrupt shared state. The lock SHALL be a sibling S3 object at `<key>.lock`
created with a conditional put (`IfNoneMatch: "*"`, atomic create-if-absent): a
successful create acquires the lock; a precondition failure means the lock is held.
The lock object SHALL contain lock information identifying the holder (a user, a
host, a pid, an ISO-8601 timestamp, the operation, and a generated lock id).
Acquiring a held lock SHALL fail with an actionable error that reads the holder's
information back (who, since when, which operation) and points at the force-unlock
escape hatch. Releasing SHALL delete the lock object; an `Unlock` that is given a
lock id SHALL refuse to delete a lock whose id does not match (so a stale release
cannot drop another run's lock), while a forced unlock SHALL delete the lock
unconditionally. The lock seam SHALL be an OPTIONAL interface a backend MAY
implement, so the `Store` interface is unchanged and a backend without locking
(e.g. the local file store today) simply does not lock.

#### Scenario: acquiring a free lock succeeds
- GIVEN an s3 backend with no lock object
- WHEN the lock is acquired
- THEN the `<key>.lock` object is created with the holder's lock information and the acquire succeeds.

#### Scenario: a held lock blocks a second acquire with the holder's info
- GIVEN an s3 backend whose lock is already held by run A
- WHEN run B tries to acquire it
- THEN the acquire fails with an error naming run A's holder (user/host), the time since when it has been held, and the operation, and pointing at force-unlock.

#### Scenario: unlock releases the lock
- GIVEN a lock held by a run
- WHEN that run unlocks with its own lock id
- THEN the lock object is removed and a subsequent acquire succeeds.

#### Scenario: unlock with a mismatched id is refused
- GIVEN a lock held with id X
- WHEN unlock is called with id Y (≠ X)
- THEN it refuses and does not delete the lock (it is not this caller's lock).

#### Scenario: force-unlock clears a held lock
- GIVEN a lock held (e.g. left by a crashed run)
- WHEN force-unlock runs
- THEN the lock object is deleted unconditionally and a subsequent acquire succeeds.

#### Scenario: a backend without locking does not lock
- GIVEN a store that does not implement the lock seam (the local file store)
- WHEN a mutating run uses it
- THEN no lock is taken and behaviour is unchanged.

