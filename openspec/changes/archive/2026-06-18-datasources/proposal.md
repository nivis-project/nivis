# Proposal: datasources

## Why
A Nivis config can only create resources; it cannot **read existing**
infrastructure. Every other IaC tool can look up an AMI by filter, a VPC by tag,
an availability zone, an existing bucket, and feed that into new resources. The
Terraform plugin protocol's `ReadDataSource` is the mechanism, and our providers
(real and fake) expose it, but the executor never calls it and the Nix library has
no way to declare a datasource. This is A2 of the "Road to v1" milestone
(beans-6e6i), a core piece of being a daily-driver.

## What changes
Datasources, end to end: declared in Nix, read by the executor per phase, their
outputs feeding resources through the same ref mechanism.

- **Nix (`mkData`):** a constructor mirroring `mkResource` for a datasource node:
  `mkData { provider; type; name; config; }` with a stable id
  `data.<provider>.<type>.<name>` and `refAttr` access to its outputs, so a
  resource can reference `someData.refAttr "id"` exactly like a resource ref.
- **IR (`dataSources` array):** the IR gains a top-level `dataSources` array,
  distinct from `resources`. A datasource node is `{ id, provider, type, name,
  config }` (no `meta`/lifecycle: datasources are read, never applied or
  destroyed). Refs and edges may target a datasource id like any other node. This
  is an IR-contract change, so the contract is updated **first** (hard gate).
- **Executor (`ReadDataSource`):** the provider `Client` interface gains
  `ReadDataSource`, plumbed through the v6 and v5 backends. The phase driver reads
  each datasource **when its config inputs are fully known**, via the *same*
  readiness loop that applies ready resources, and puts its returned attributes in
  the outputs ledger. So a datasource whose config is fully known reads in phase 0,
  and one that depends on a resource's apply-time output reads in a later phase:
  datasources participate in the fixpoint. A datasource is never planned, applied,
  destroyed, or written to state.
- **Fake provider:** a datasource-serving fake (extending the existing fake
  provider machinery) so the whole path is tested hermetically (no network, no
  credentials), per `docs/TESTING.md`.

## Decisions (settled with the maintainer)
- **Separate `dataSources` array**, not a `mode:"data"` flag on resources. Keeps
  the apply/destroy/state paths clean; a datasource is a genuinely different node.
- **Per-phase read**, reusing the existing readiness loop, not an up-front pass.
  It is a strict superset (simple lookups still read in phase 0) and it preserves
  the round trip: a datasource may depend on a resource's apply-time output.

## Non-goals
- **Datasource-specific schema codegen** (`nivis gen` emitting typed `mkData`
  constructors). Deferred; `mkData` is hand-usable now, like `mkResource` was
  before codegen.
- **Caching / refresh semantics** beyond "read once per run when ready". A
  datasource is re-read on a fresh run; there is no staleness policy yet.
- **`depends_on` on a datasource.** Implicit refs are enough for v1; explicit
  data `depends_on` can come later.
- **Sensitive datasource outputs** beyond the existing sensitive-value handling
  (a datasource output marked sensitive by the provider follows the same ledger
  rules as a resource output).

## Impact
- IR: `docs/IR-CONTRACT.md` (new `dataSources` section); `docs/ir-schema.json`
  (optional `dataSources` array); the conformance suite gains datasource fixtures.
  `internal/ir` (`Document.DataSources`, a `DataSourceNode`, ingest + ref
  classification + graph wiring).
- Provider: `internal/provider` (`Client.ReadDataSource`, request/result types),
  `internal/provider/v6` and `internal/provider/v5` (proto plumbing).
- Executor: `internal/phase` (read ready datasources in the loop, feed the
  ledger; fixpoint/stuck rules include datasources).
- Nix: `nix/lib/mkData.nix` (+ export); property test.
- Fake: a datasource-serving fake provider; integration test proving a datasource
  read feeds a resource, including the dependent (later-phase) case.
- Docs: **new document `docs/DATASOURCES.md`** (+ docs-site page + SUMMARY entry);
  README library list gains `mkData`.

Docs impact: new document, docs/DATASOURCES.md (the canonical reference for
reading existing infrastructure with mkData and how datasources read per phase),
surfaced on the docs site (docs-site/src/datasources.md + SUMMARY entry); README
library list gains mkData. Datasources are a noun users search for by name, so
they warrant their own page (docs/DOCS-GATE.md rubric).
