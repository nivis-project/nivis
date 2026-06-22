# Proposal: s3-state-backend

## Why
M2/B1 (the user's #1 priority: "remote state using s3"). The `Store` interface is
the seam and the IR now carries an optional `backend` block (`ir-backend-block`).
This change implements the S3 backend behind that seam, so a team and CI can keep
state in a shared S3 object instead of a local file. State stays Nivis's own format
(NO tfstate compatibility, per DESIGN); only the storage location moves.

## What changes
- **An S3-backed `Store`** (`internal/state`): the whole state **document** is one
  S3 object (the canonical JSON `Snapshot`/`Restore` bytes, the same document the
  local store and `state pull`/`push` use). Per-resource `Get`/`Set`/`Delete`/`List`
  are implemented as read-modify-write of that object (load document, apply,
  put document), matching the local store's document semantics.
  - Server-side encryption (`SSE` = `AES256`) on every put.
  - Credentials from the **AWS default credential chain** (env, profile, instance
    role) via the SDK — never from config. Only the location (`bucket`/`key`/
    `region`) comes from the IR `backend`.
  - An optional `endpoint` override (and path-style addressing) so the backend can
    be pointed at a test/S3-compatible server. In production it is unset and the
    SDK resolves the real S3 endpoint.
- **Backend selection** wired through the executor: when the IR declares
  `backend.type == "s3"`, the CLI opens the S3 store (from `bucket`/`key`/`region`/
  optional `endpoint`) instead of the local file store; absent/local keeps today's
  behaviour. A small `state.OpenBackend(backend)` constructor maps an IR backend
  object to a `Store`, so plan/apply/destroy/refresh/state-commands all use it
  unchanged (they already depend only on the `Store` interface).
- **An in-repo fake S3** (`internal/fakes3` or under the existing test substrate):
  an `httptest` server speaking the few S3 verbs we use (PutObject, GetObject,
  DeleteObject, ListObjectsV2 / HEAD), backed by an in-memory map. Hermetic, no
  network, no credentials. It is the S3 analogue of the fake providers.
- **AWS SDK for Go v2** is added as a dependency (`config`, `service/s3`,
  `credentials`), fetched via the Go proxy (verified reachable; the registry is
  not needed).

## Decisions (settled with the maintainer)
- **Real SDK against a configurable endpoint, hermetic fake S3 in tests** (not an
  abstract object-store interface): the production code path (the real SDK) is the
  one exercised by tests, pointed at the in-repo fake.
- **Backend declared in the flake** via the IR `backend` block (done in
  `ir-backend-block`); credentials from the AWS chain.

## Non-goals
- **State locking** (B2): a separate epic/change. This change ships the S3 store
  without a distributed lock; concurrent applies are addressed by B2. (The S3
  store keeps the document read-modify-write atomic per call, but two concurrent
  applies are not yet serialized; B2 adds that.) This limitation is noted in docs.
- tfstate compatibility (explicitly not a goal, DESIGN).
- Drift detection (B3), multiple environments (B4).
- A nix-built AWS path: the SDK talks HTTPS to S3 directly; no provider spawn.

## Impact
- `go.mod`/`go.sum`: AWS SDK for Go v2.
- `internal/state`: `s3Store` implementing `Store` over one S3 object;
  `OpenBackend(backend map[string]interface{}) (Store, error)` selecting the impl
  from an IR backend block; the local store stays the default.
- `internal/fakes3`: the hermetic in-memory S3 `httptest` server for tests.
- `cmd/nivis`: open the store from the IR `backend` when present (a phase-0 eval
  already yields the IR; reuse it to learn the backend), else the local `--state`
  file. Clear error if `backend.type` is unknown.
- Docs: a **new `docs/REMOTE-STATE.md`** (a user searches for "remote state" by
  name) covering the `nivis.backend = { type = "s3"; ... }` flake usage, the AWS
  credential chain, SSE, and the "no locking yet (B2)" caveat; surfaced on the site
  (`docs-site` include + SUMMARY entry).
- Tests: unit tests for `s3Store` against the fake S3 (round-trip a document;
  Get/Set/Delete/List; SSE header sent; Snapshot/Restore), a backend-selection
  test (`OpenBackend` picks s3 vs local; unknown type errors), and a hermetic
  **e2e**: a full plan/apply/output cycle with `backend = s3` pointed at the fake
  S3, asserting state lands in the S3 object and a re-plan reads it back.

Changelog: Added an S3 remote state backend: declare `nivis.backend = { type =
"s3"; bucket; key; region; }` in the flake and state is stored in that S3 object
(server-side encrypted, AWS credential chain, Nivis's own format, no tfstate
compatibility). Locking is a follow-up (B2).

Docs impact: new document - `docs/REMOTE-STATE.md` (remote state / the s3 backend
is a capability a user searches for by name), surfaced on the site. Per
docs/DOCS-GATE.md.
