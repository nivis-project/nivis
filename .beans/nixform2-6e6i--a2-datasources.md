---
# nixform2-6e6i
title: 'A2: Datasources'
status: completed
type: epic
priority: high
tags:
    - roadmap
created_at: 2026-06-16T13:37:59Z
updated_at: 2026-06-18T06:00:11Z
parent: nixform2-zdj0
---

Read existing infrastructure (an AMI by filter, a VPC, an AZ) and feed it into resources, the way every other IaC tool can.

ROADMAP Phase A2. GROUND TRUTH: the provider protocol method ReadDataSource exists on our fakes/real providers but the executor never drives it, and there is no datasource constructor in nix/lib.

## Scope
- Nix-lib constructor (mkData or similar) for a datasource node.
- Executor drives ReadDataSource per phase, like any other node; outputs feed refs.
- IR-CONTRACT.md addition for the datasource node shape and its outputs (OpenSpec change to the contract FIRST).
- A datasource-serving fake provider for hermetic tests.

Datasource reads participate in the phased fixpoint (a data lookup may depend on an earlier resource's output).


---
## Summary of Changes
DONE via OpenSpec change datasources (archived 2026-06-18-datasources). Read existing infrastructure end to end:

- NIX: nivis.mkData { provider; type; name; config; } mirrors mkResource, id "data.<p>.<t>.<n>", refAttr/refPath. toIR gains a dataSources param and emits a dataSources array; refs to/from a datasource produce edges (collectEdges is target-agnostic). Exported from default.nix.
- IR: optional top-level dataSources array (node = {id,provider,type,name,config}, no meta). IR-CONTRACT.md + ir-schema.json (new dataSource $def, data.-prefixed id pattern) + conformance (check.py includes datasource ids as valid ref/edge targets; 2 new fixtures valid+invalid). The frozen-contract gate.
- PROVIDER: Client gains GetDataSourceSchema + ReadDataSource, plumbed through v6 and v5 (encode config, call ReadDataSource, decode state). Test mocks updated.
- EXECUTOR: the phase driver reads a datasource via readOne when its config is fully known, REUSING the existing readiness loop (dispatch on Resource.IsData: read vs apply). Datasources are excluded from PlanReport; never written to state, so destroy/refresh (state-membership gated) skip them automatically. Fixpoint/stuck detection covers them (they share g.Order).
- FAKE: provider-alpha serves an alpha_lookup datasource (deterministic: result="found:<query>"); fakeprovider gained WithDataSources + a real ReadDataSource.

DECISIONS (with maintainer): separate dataSources array (not a mode flag); per-phase read reusing the loop (not up-front) - a strict superset that preserves the round trip.

KEY DESIGN: datasource nodes live in the SAME g.Nodes/g.Order as resources (marked IsData), so ResolveTFTF, readiness, fixpoint and stuck-detection all work UNCHANGED; only the driver dispatch and the read-vs-apply/plan/state/destroy exclusions differ. This is why per-phase cost ~= up-front.

TESTS: Nix property P10 (mkData id/refAttr/edge/in-dataSources). Go: ir ingest, v5/v6 ReadDataSource via the interface. Integration against the REAL fake binary: a datasource reads and feeds a resource (not in state); AND the round-trip case (resource A applies -> datasource reads from A.value in a later phase -> feeds resource B, 3 phases). Full gate green: gofmt, go build, go test, run-nix-tests (P10 + conformance 9/9), check-docs-ssot (SSOT + comparison-fresh + docs-coverage + mdbook).

DOCS (docs-coverage gate: new document): docs/DATASOURCES.md (mkData, when reads happen per phase, the dependent case, resource-vs-datasource table, non-goals); docs-site/src/datasources.md + SUMMARY; README mkData line. No em dashes.

NON-GOALS (deferred): datasource codegen (nivis gen typed mkData); caching/staleness; data depends_on; sensitive beyond existing handling.


---
Follow-up question (2026-06-18): "don't we need outputs to complete datasources?" Answer: NO. A datasource's attributes already flow back IN (mkData refAttr -> ledger -> resources/Nix; the round trip works). The OTHER meaning of "outputs" (named values surfaced OUT of the run, the Terraform `output {}` equivalent: `nivis output` + a published artifact) is a separate, orthogonal gap that applies to resources and datasources alike. Filed as epic nixform2-h9ws (A7) under the same milestone. Datasources ship complete.
