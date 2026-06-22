---
# nixform2-izhk
title: 'B1: Remote state backend (S3 first)'
status: completed
type: epic
priority: normal
tags:
    - roadmap
created_at: 2026-06-16T13:38:17Z
updated_at: 2026-06-22T14:40:30Z
parent: nixform2-kovh
---

Implement the Store seam against S3. The user's #1 personal priority: "remote state using s3".

GROUND TRUTH: internal/state has a Store interface; only local JSON is implemented.

## Scope
- S3 backend: object per state, server-side encryption, the AWS credential chain Nivis already uses.
- Format stays Nivis's own (NO tfstate compatibility, DESIGN).
- Configured in the flake, not via env soup.
- Reuses the A6 state pull/push shape.



## OpenSpec changes
- ir-backend-block (archived 2026-06-22-ir-backend-block): the optional IR `backend` field + toIR/toModuleIR pass-through + Go ingest/validation (static, type required, no refs). Contract groundwork.
- s3-state-backend (next): the S3 Store impl (AWS SDK + endpoint), in-repo fake S3, CLI backend selection, hermetic e2e.



## Summary of Changes (B1 delivered, 2026-06-22)

Two OpenSpec changes, both archived:
- ir-backend-block (2026-06-22-ir-backend-block): optional IR `backend` field + toIR/toModuleIR pass-through + Go ingest/validation (static, type required, no refs).
- s3-state-backend (2026-06-22-s3-state-backend): the S3 Store.

S3 backend:
- internal/state/s3.go: s3Store implements the Store interface over ONE S3 object holding the canonical state document (shared document.go helpers with the local store). Get/Set/Delete/List = read-modify-write of that object; Snapshot/Restore = the pull/push seam. Every PutObject sets SSE=AES256. Credentials from the AWS default chain (config.LoadDefaultConfig); only bucket/key/region/optional-endpoint from the backend. Missing object reads as empty.
- internal/state/backend.go: OpenBackend(backend, localPath) selects s3 vs local; validates required s3 keys and unknown type with actionable errors.
- internal/fakes3: hermetic in-memory S3 httptest server (Put/Get/Delete/HEAD, path-style) — the S3 analogue of the fake providers. No network, no creds.
- cmd/nivis: openStore(ctx) reads the IR backend (phase-0 eval) and opens the selected store; tolerant fallback to local when the config cannot be evaluated (so state pull/push/completion in a bare dir still work). All state-using commands operate through the Store interface unchanged.
- AWS SDK for Go v2 added (config/credentials/service/s3), via the Go proxy.

Proof:
- internal/state: s3Store round-trip/delete/snapshot-restore against the fake S3 (SSE asserted); OpenBackend selection + error tests.
- tests/e2e TestS3BackendRoundTrip: full apply through the driver stores state in the fake S3 (SSE verified, nothing local), a fresh store reads it back, and a re-plan reports all no-op.
- Docs: new docs/REMOTE-STATE.md (surfaced on the site) covering nivis.backend s3 usage, the AWS credential chain, SSE, no-tfstate, and the 'no locking yet (B2)' caveat.

gofmt / go build ./... / go test ./... / nix tests / docs-ssot gate all green.

## Known limitation -> B2
No distributed lock yet: two concurrent applies of the same stack can race. State locking is nixform2-0oqk (B2), the next M2 epic.
