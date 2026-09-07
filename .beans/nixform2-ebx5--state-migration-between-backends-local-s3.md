---
# nixform2-ebx5
title: State migration between backends (local <-> S3)
status: in-progress
type: feature
priority: high
created_at: 2026-09-07T15:23:04Z
updated_at: 2026-09-07T16:23:45Z
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
