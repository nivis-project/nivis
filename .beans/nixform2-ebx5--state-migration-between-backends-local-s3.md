---
# nixform2-ebx5
title: State migration between backends (local <-> S3)
status: completed
type: feature
priority: high
created_at: 2026-09-07T15:23:04Z
updated_at: 2026-09-07T16:46:53Z
parent: nixform2-kovh
---

`nivis` cannot move an existing state document from one backend to another. `state.OpenBackend` (internal/state/backend.go) selects a store straight from the IR `backend` block and opens it; nothing copies a pre-existing local document into a newly declared remote backend, and there is no verb to do it by hand.

## Why this matters

It makes the standard "self-managed state bucket" bootstrap impossible. A domain that creates the very bucket it declares as its own backend cannot be applied:

- `cmd/nivis/main.go` `openStore` evaluates phase 0 only to *discover* the backend, then opens it — so an s3 backend is opened on the first apply, before the bucket exists.
- `internal/state/s3.go` `isNotFound` matches only `NoSuchKey` / `NotFound` / `404`. A missing *bucket* returns `NoSuchBucket`, so the state read errors instead of starting from an empty document, and the apply fails before creating anything.

Both `infra` (stack/000_backend) and `nivis-demos` assume the documented bootstrap "first apply uses local state, then state migrates into the bucket". That assumption is false today; `infra`'s README states it as unverified, and it is wrong.

## Acceptance criteria

- A way to migrate a state document between backends (e.g. `nivis state migrate`, or an automatic local -> declared-remote move on first run), with the destination refusing to clobber a non-empty existing document unless forced.
- The self-managed-bucket bootstrap works: applying a domain that creates its own state bucket succeeds from a clean checkout and ends with state in the bucket.
- Decide and document whether `NoSuchBucket` should count as an empty document (which would make the one-apply bootstrap work directly) or stay an error.
- Round-trip covered by tests, including S3 -> local.

## Context

Found while implementing the `catstack-scaffold-and-state-backend` OpenSpec change in `nivis-demos`, where the 000_backend domain hit exactly this wall.

## Summary of Changes

Shipped as OpenSpec change `state-migrate-between-backends` (archived:
`openspec/changes/archive/2026-09-07-state-migrate-between-backends/` in the
`nivis` store). Commit `d8a3005`.

**What was built**

- `nivis state migrate --to-remote` / `--from-remote` (`cmd/nivis/statemigrate.go`):
  moves the whole state document between the local file store and the declared
  backend. Locks both sides (destination first, a fixed order so opposing
  migrations fail instead of deadlocking), copies, **verifies** by reading the
  destination back, and only then removes the source.
- `state.Migrate` + the destination guard (`internal/state/migrate.go`), keyed on
  content rather than object existence: absent/empty proceeds, an identical
  document **resumes** an interrupted migration, differing resources or
  unparseable content is refused unless forced.
- The optional `Remover` seam (`internal/state/remove.go`): `Store.Delete(id)`
  removes one resource, so whole-document removal needed its own interface,
  shaped like the existing `Locker`. The local store also clears its derived
  `.ledger`/`.lock` siblings; the S3 store deletes only its state object.
- `--backend=local` (`cmd/nivis/main.go`): a per-run override of backend
  selection, announced whenever it contradicts a declared backend.
- The `NoSuchBucket` decision: a missing **object** still reads as an empty
  stack; a missing **bucket** is now a `MissingBucketError` naming bucket and
  region and carrying the bootstrap recipe. `AccessDenied` keeps reporting its
  own cause.

**The finding that reshaped the work**

The bean blamed `isNotFound` not matching `NoSuchBucket`. That is real but not
where the bootstrap fails: `apply` is `openStore` → `withStateLock` → `Run`, and
because the S3 store is a `Locker`, an apply's FIRST S3 call is `PutObject
<key>.lock`. Widening the read classification alone would have left `apply`
dying one step earlier. `TestS3MissingBucketOnLockAcquisition` pins that.

Because state is persisted per resource as an apply progresses, "one apply
straight into a bucket it creates" would require buffering writes until the
bucket exists — a crash mid-buffer means created resources with no state record.
That is why the shipped bootstrap is the explicit two-apply sequence.

**Acceptance criteria**

- Migration with a clobber guard: done (`state migrate`, guard table, `--force`).
- Self-managed-bucket bootstrap works from a clean checkout: done, covered by
  `tests/e2e/bootstrap_state_test.go` `TestSelfManagedBucketBootstrap` (real CLI,
  fake providers, hermetic fake S3).
- `NoSuchBucket` decided and documented: it is an **error**, not empty state —
  docs/REMOTE-STATE.md "A missing bucket is an error, not empty state".
- Round trip covered including S3 → local: `TestMigrateRoundTripLocalToS3AndBack`.

**Docs**: docs/REMOTE-STATE.md gained "Moving state between backends",
"Bootstrapping a self-managed state bucket", and the missing-bucket rule;
docs/TUTORIAL-REMOTE-STATE.md's migration note was updated.

**Discovered work filed**: `nixform2-j02d` (`Store.List()` errors swallowed at
`internal/phase/driver.go:218`/`:339`), `nixform2-ih51` (automatic local-first
bootstrap detection from the phase-0 IR graph — deferred design decision 5).
