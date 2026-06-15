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

