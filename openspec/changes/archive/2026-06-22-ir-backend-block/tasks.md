# Tasks: ir-backend-block

## 1. Contract
- [x] 1.1 `docs/IR-CONTRACT.md`: document the optional top-level `backend` field
      (static config; `type` required; no refs/unknowns; absent = local store;
      credentials never here). Note it is additive, schemaVersion stays 1.
- [x] 1.2 `docs/ir-schema.json`: add `backend` as an optional object with a
      required `type` string (additionalProperties allowed for backend-specific
      keys, validated by the backend, not the schema).

## 2. Nix library
- [x] 2.1 `nix/lib/toIR.nix`: accept an optional `backend ? null` argument and
      emit `backend` in the IR only when non-null.
- [x] 2.2 `nix/lib/modules.nix` (`toModuleIR`): thread an optional `backend`
      through the same way, so a module-composed config can declare it.

## 3. Go ingest + validation
- [x] 3.1 `internal/ir/types.go`: add `Backend map[string]interface{}
      json:"backend,omitempty"` to `Document` (and surface it on `Graph` if the
      executor needs it).
- [x] 3.2 `internal/ir/ingest.go`: parse `backend`; validate when present that
      `type` is a non-empty string and that NO leaf is a `__ref`/`__derived`/
      unknown (reuse the existing ref-detection), failing with an actionable error
      naming the offending path. Absent backend is fine.

## 4. Tests
- [x] 4.1 Nix: a property/eval test that `toIR` with a `backend` emits it verbatim,
      and without one omits the field; add to `tests/run-nix-tests.sh` conformance.
- [x] 4.2 Go: `internal/ir` table test — a valid s3-shaped backend parses; a
      backend missing `type` is rejected; a backend with a `__ref` leaf is rejected
      with the path named; absent backend yields a nil/empty backend.
- [x] 4.3 Conformance: an IR with a backend validates against `docs/ir-schema.json`.

## 5. Changelog
- [x] 5.1 `CHANGELOG.md` `[Unreleased]` `### Added`: the IR `backend` field
      (matches the proposal's `Changelog:` line).

## 6. Gate
- [x] 6.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 6.2 `bash tests/run-nix-tests.sh` + `bash tests/check-docs-ssot.sh` green.
- [x] 6.3 `openspec validate ir-backend-block --strict`; archive. (Bean stays open:
      B1 is delivered by the follow-up s3-state-backend change.)
