# Tasks: s3-state-backend

## 1. AWS SDK dependency
- [x] 1.1 Add aws-sdk-go-v2 (`config`, `credentials`, `service/s3`) to go.mod via
      the Go proxy; `go mod tidy`. (Verified reachable; registry not needed.)

## 2. S3 Store
- [x] 2.1 `internal/state/s3.go`: `s3Store` implementing the `Store` interface over
      ONE S3 object holding the canonical state document (the Snapshot/Restore
      bytes). `Get/Set/Delete/List` = read-modify-write of that object; `Snapshot`
      = GetObject (empty/missing => empty document); `Restore` = validate then
      PutObject. Every put sets server-side encryption (AES256).
- [x] 2.2 Credentials via the AWS default chain (`config.LoadDefaultConfig`);
      `bucket`/`key`/`region` from the backend; optional `endpoint` (+ path-style)
      for tests/S3-compatible servers. No credentials ever from config.
- [x] 2.3 `state.OpenBackend(backend map[string]interface{}) (Store, error)`:
      `type=="s3"` (or nil/local) selection; validate required s3 keys
      (bucket/key/region) with actionable errors; unknown type errors clearly.

## 3. In-repo fake S3 (hermetic test substrate)
- [x] 3.1 `internal/fakes3`: an `httptest`-based S3 server speaking the verbs the
      store uses (PutObject, GetObject, DeleteObject, ListObjectsV2, HEAD),
      in-memory map backed. Returns S3-shaped XML/responses the SDK accepts.
      Exposes its URL for the `endpoint` override. No network, no creds.

## 4. CLI wiring
- [x] 4.1 `cmd/nivis`: when the evaluated IR declares a `backend`, open the store
      via `state.OpenBackend` instead of the local `--state` file; absent => local
      (unchanged). Plan/apply/destroy/refresh/state commands use the resulting
      Store unchanged (they depend only on the interface).
- [x] 4.2 A clear error if `backend.type` is unsupported or required s3 keys are
      missing.

## 5. Tests
- [x] 5.1 `internal/state`: `s3Store` unit tests against the fake S3 — Set/Get a
      resource; List; Delete; Snapshot/Restore round-trip; an SSE header is sent;
      a fresh (missing object) store reads as empty.
- [x] 5.2 `OpenBackend` selection test: s3 backend -> s3Store; nil/local ->
      file store; unknown type and missing bucket/key/region error clearly.
- [x] 5.3 Hermetic e2e (`tests/e2e`): a config with `backend = { type="s3"; ... }`
      pointed at the fake S3 (via endpoint); a full apply stores state in the S3
      object, and a re-plan reads it back (reports no-op), with state NOT written
      to a local file.

## 6. Docs
- [x] 6.1 New `docs/REMOTE-STATE.md`: the `nivis.backend` s3 usage, AWS credential
      chain, SSE, Nivis-own-format/no-tfstate, and the "no locking yet (B2)"
      caveat. No em dashes.
- [x] 6.2 Surface it on the site: `docs-site/src/remote-state.md` `{{#include}}`
      + a `SUMMARY.md` entry.

## 7. Changelog
- [x] 7.1 `CHANGELOG.md` `[Unreleased]` `### Added`: the S3 backend (matches the
      proposal's `Changelog:` line).

## 8. Gate
- [x] 8.1 `gofmt`, `go build ./...`, `go test ./...` green.
- [x] 8.2 `bash tests/run-nix-tests.sh` + `bash tests/check-docs-ssot.sh` green.
- [x] 8.3 `openspec validate s3-state-backend --strict`; archive; update beans-izhk
      (B1 delivered: IR block + S3 store) and note B2 (locking) is next; if B1 is
      the only remaining epic-as-todo blocker, leave M2 in-progress for B2-B4.
