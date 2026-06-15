# Proposal: executor-core

## Why
The Go executor is pure orchestration: it ingests the frozen IR, builds the
dependency graph, resolves the references it can resolve itself, and persists
state. This change establishes that core — everything the executor does that does
**not** require talking to a provider — as a tested foundation the plugin client
and plan/apply engines (a following change) build on.

Splitting E3 here is deliberate (conventions: small, independently reviewable
changes). The graph/ingestion/state core is pure, fully testable offline, and is
the substrate the phased-eval loop (E3.5) drives. The plugin client requires
generated `tfplugin6` gRPC stubs (a separate, bounded concern — see beans
nixform2-uu26), so it is its own change.

## What changes
- `IngestIR`: read + validate IR JSON against the contract
  (`docs/IR-CONTRACT.md` / `docs/ir-schema.json`), producing typed
  `ResourceNode`, `ProviderConfig`, `MetaArgs`, and a `RefEdge` list. Malformed
  IR is rejected with an error naming the offending resource/path.
- Ref classification: each ref/derived leaf is tagged **TF→TF** (in
  `resources[].config`, resolvable in-executor) or **\*→Nix** (in
  `nixConsumers[]` or a `__derived` leaf; resolvable only by re-eval). Matches
  the IR contract's classification.
- DAG builder: a dependency graph from `RefEdge` + `meta.dependsOn`, with cycle
  detection and a deterministic topological/ready-set traversal that honors
  `depends_on`.
- TF→TF resolution: given a map of known resource outputs, substitute resolved
  values into dependent resources' configs; report which resources are now fully
  known vs still pending.
- State backend: a lockable local JSON state store (get/set/list/delete of
  resource states; advisory file lock). **No tfstate-format compatibility**
  (DESIGN: trivial state until the round trip works); interface designed so
  remote backends can come later.

## Non-goals
- The provider plugin client / spawn / handshake — needs generated gRPC stubs;
  next change (beans nixform2-uu26).
- Plan and apply engines — they call the provider; next change.
- Phased re-eval loop (E3.5), CLI, refresh/destroy (E3b) — later epics/changes.
- `__ref` → tfprotov6 unknown mapping — belongs with the plan engine (it needs
  the provider value types); this change only classifies and resolves refs at
  the IR level.

## Impact
- New: `internal/ir/` (types + ingest/validate), `internal/graph/` (DAG +
  resolution), `internal/state/` (JSON state backend).
- Establishes `ResourceNode`/`RefEdge`/`ProviderConfig` — the in-memory model the
  plugin client and plan/apply engines consume next, and that the phased loop
  (E3.5) queries for "which resources are ready this phase".
