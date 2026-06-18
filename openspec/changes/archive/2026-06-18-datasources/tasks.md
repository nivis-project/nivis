# Tasks: datasources

## 1. IR contract (the frozen-contract gate, first)
- [x] 1.1 `docs/IR-CONTRACT.md`: a `## Datasources (`dataSources`)` section: node
      shape `{ id, provider, type, name, config }`, id form `data.<p>.<t>.<n>`,
      no meta/lifecycle, refs/edges may target a datasource, optional array.
- [x] 1.2 `docs/ir-schema.json`: optional top-level `dataSources` array of
      datasource-node objects (additive; existing IR stays valid).
- [x] 1.3 `tests/ir-conformance/`: a valid fixture with a datasource (+ a
      resource referencing it) and an invalid one (duplicate/ill-formed id);
      `check.py test` green.

## 2. IR ingestion (Go)
- [x] 2.1 `internal/ir`: `Document.DataSources []DataSource`; a `DataSourceNode`
      (or reuse the node shape) on the `Graph`; ingest validates unique ids,
      declared provider, classifies refs, and wires edges (a datasource is a
      graph node refs can target and that can ref others).
- [x] 2.2 Unit tests for ingest: a datasource node is parsed, ref-classified, and
      a ref to/from it produces the right edge; a duplicate id errors.

## 3. Provider client (Go)
- [x] 3.1 `internal/provider`: `Client.ReadDataSource(ctx, req) (result, error)`
      with request/result types; `ReadDataSourceRequest{TypeName, Config}`,
      `ReadDataSourceResult{State, Diagnostics}`.
- [x] 3.2 `internal/provider/v6`: implement via tfprotov6 `ReadDataSource`,
      encoding config and decoding state with the existing codec + datasource
      schema (from GetProviderSchema's data-source schemas).
- [x] 3.3 `internal/provider/v5`: same over tfprotov5.

## 4. Phase driver (Go)
- [x] 4.1 `internal/phase`: in the readiness loop, a ready datasource (config
      fully known) is READ (not planned/applied); its returned attrs go into the
      ledger keyed by its id. Read at most once.
- [x] 4.2 Fixpoint/stuck detection includes datasources: an unread datasource at
      fixpoint whose inputs never resolved is a stuck-node error naming it.
- [x] 4.3 Datasources are excluded from plan/apply/destroy/state paths.

## 5. Nix: mkData
- [x] 5.1 `nix/lib/mkData.nix`: `mkData { provider; type; name; config; }` ->
      `{ id = "data.<p>.<t>.<n>"; ... refAttr; }`, mirroring mkResource.
- [x] 5.2 `nix/lib/toIR.nix`: collect datasources into the `dataSources` array;
      a ref to a datasource id produces an edge.
- [x] 5.3 Export `mkData` from `nix/lib/default.nix`.
- [x] 5.4 `nix/tests/properties.nix`: a property covering mkData id, refAttr ->
      __ref + edge, and toIR placing it in `dataSources`.

## 6. Fake provider + integration
- [x] 6.1 A fake provider declares a datasource type and implements
      `ReadDataSource` with deterministic attrs from config (extend the existing
      fake machinery; the fakeproviderx datasource stub becomes real, or add to a
      fake).
- [x] 6.2 Integration (StubEvaluator or real flake) against the fake binary: a
      datasource reads and its output feeds a resource; AND the dependent case (a
      datasource whose config needs a resource output reads in a later phase).

## 7. Docs (the docs-coverage gate: new document)
- [x] 7.1 `docs/DATASOURCES.md`: canonical reference (mkData, when reads happen
      per phase, the dependent case, outputs feeding resources, non-goals).
- [x] 7.2 `docs-site/src/datasources.md` `{{#include}}`s it; add to SUMMARY.md.
- [x] 7.3 README Nix-library list: add `mkData`. (No em dashes.)

## 8. Gate
- [x] 8.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 8.2 `bash tests/run-nix-tests.sh` (properties + IR conformance) green.
- [x] 8.3 `bash tests/check-docs-ssot.sh` green (SSOT + comparison-fresh +
      docs-coverage + mdbook build).
- [x] 8.4 `openspec validate datasources --strict`; archive; close beans-6e6i.
