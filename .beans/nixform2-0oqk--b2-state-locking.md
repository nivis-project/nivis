---
# nixform2-0oqk
title: 'B2: State locking'
status: completed
type: epic
priority: normal
tags:
    - roadmap
created_at: 2026-06-16T13:38:17Z
updated_at: 2026-06-22T15:20:51Z
parent: nixform2-kovh
---

A lock so two concurrent applies cannot corrupt shared state.

## Scope
- DynamoDB-style advisory lock for the S3 backend.
- `force-unlock` escape hatch.
- Clear "who holds the lock / since when" errors.
Depends on B1.



## Summary of Changes (B2 delivered, 2026-06-22, OpenSpec change s3-state-locking)

S3-native advisory lock, no DynamoDB:
- internal/state/lock.go: optional Locker interface (Lock(LockInfo)->id, Unlock(id), ForceUnlock) + LockInfo (who/host/pid/operation/created/id) and NewLockInfo(operation). The Store interface is unchanged; a backend that does not implement Locker is simply unlocked.
- internal/state/s3.go: s3Store implements Locker via a sibling <key>.lock object created with a conditional put (IfNoneMatch:'*', atomic create-if-absent). A 412 PreconditionFailed means held -> reads the holder back and errors ('state is locked by <who> since <when> for <op>; run nivis force-unlock'). Unlock refuses to delete a lock whose id differs (no dropping someone else's lock); ForceUnlock deletes unconditionally.
- internal/fakes3: honors If-None-Match:'*' (412 when present) so the lock is hermetically testable.
- cmd/nivis: withStateLock wraps apply and destroy (acquire before, release deferred after, including on failure; prints Acquired/Released). Read-only commands do not lock. New 'nivis force-unlock' command (confirm prompt, --force/--yes; clear message; a non-Locker backend reports nothing to clear).

Proof:
- internal/state lock unit tests against the fake S3 (acquire/block-with-holder-info/release; wrong-id refused; force-unlock; re-acquire).
- cmd/nivis TestForceUnlockNonLockerBackend.
- tests/e2e TestS3StateLockMutualExclusion: run A holds, run B rejected with A's holder info, release -> B acquires, force-unlock clears.
- Docs: docs/REMOTE-STATE.md locking section (replaces the 'no locking yet' caveat).

gofmt / go build ./... / go test ./... / nix tests / docs-ssot gate all green.

Next in M2: B3 drift detection (nixform2-tyzs), B4 multiple environments (nixform2-cdfj).
