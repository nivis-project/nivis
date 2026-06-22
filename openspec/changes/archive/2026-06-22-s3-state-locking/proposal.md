# Proposal: s3-state-locking

## Why
M2/B2. The S3 backend (B1) lets a team share state, but nothing stops two applies
from running at once and corrupting it (the documented B1 limitation). This change
adds an advisory **lock** so a mutating run holds the state exclusively, with a
`force-unlock` escape hatch and clear "who holds the lock / since when" errors.

## What changes
- **An optional `Locker` seam** (`internal/state`): a small interface a backend MAY
  implement:
  ```go
  type Locker interface {
      Lock(info LockInfo) (lockID string, err error)
      Unlock(lockID string) error
      ForceUnlock() error
  }
  ```
  `LockInfo` records who/where/when/operation (user, host, pid, an ISO timestamp,
  and the operation e.g. "apply"). The `Store` interface is unchanged; a backend
  that does not implement `Locker` is simply unlocked (today's behaviour).
- **An S3 conditional-put lock** (`s3Store` implements `Locker`): the lock is a
  sibling object at `<key>.lock`, created with `IfNoneMatch: "*"` (atomic
  create-if-absent, supported by S3 and the SDK). The lock body is the `LockInfo`
  JSON. Acquisition that hits a `PreconditionFailed` (412) means the lock is held:
  the error reads back the holder's info ("locked by <user>@<host> since <time> for
  <operation>; run `nivis force-unlock` to override"). `Unlock` deletes the lock
  object (only when the held id matches, so a stale unlock cannot drop someone
  else's lock); `ForceUnlock` deletes it unconditionally.
- **The CLI acquires the lock around mutating runs** (`apply`, `destroy`): if the
  selected store is a `Locker`, acquire before the run and release after (deferred,
  so a failure still unlocks), printing a short "Acquired/Released state lock"
  line. Read-only commands (`plan`, `refresh`, `state pull`, `output`) do NOT lock.
- **A `nivis force-unlock` command**: removes a stuck lock (after a crashed run),
  with a confirmation prompt (and `--force`/`--yes` for non-interactive), and a
  clear message naming what was unlocked.
- **The fake S3 gains conditional-put + delete semantics** so the lock is testable
  hermetically: `PutObject` with `IfNoneMatch: "*"` returns 412 when the object
  exists; otherwise it creates it.

## Decisions (settled with the maintainer)
- **S3 conditional-put lock object**, not DynamoDB: S3 conditional writes make the
  classic S3+DynamoDB pattern redundant; staying S3-only avoids a second service,
  extra IAM, and more test surface.
- **An optional `Locker` interface** on the backend (not folded into `Store`), so
  the `Store` contract and every existing store/test double stay unchanged.

## Non-goals
- Locking the **local** file store beyond its existing per-op advisory flock (the
  local store is single-machine; B2 targets the shared S3 backend). The local store
  may implement `Locker` later, but this change does not require it.
- Lock leases/auto-expiry/heartbeats (a crashed run leaves a lock that
  `force-unlock` clears; an expiry policy is a possible later refinement).
- Drift detection (B3), multiple environments (B4).

## Impact
- `internal/state`: the `Locker` interface + `LockInfo`; `s3Store.Lock/Unlock/
  ForceUnlock` via the `<key>.lock` object; a constructor for `LockInfo` from the
  environment (user/host/pid/time/operation).
- `internal/fakes3`: honor `IfNoneMatch: "*"` (412 when present) so the lock is
  hermetically testable.
- `cmd/nivis`: acquire/release the lock around `apply` and `destroy` when the store
  is a `Locker`; a new `force-unlock` command.
- Docs: extend `docs/REMOTE-STATE.md` (replace the "no locking yet" caveat with the
  locking behaviour: automatic on apply/destroy, the holder error, `force-unlock`).
- Tests: `s3Store` lock unit tests against the fake S3 (acquire succeeds; a second
  acquire fails with the holder info; unlock releases; force-unlock clears; unlock
  with the wrong id is refused); a `force-unlock` CLI test; an e2e that a second
  concurrent apply is rejected while a lock is held, then succeeds after release.

Changelog: Added state locking for the S3 backend: `apply`/`destroy` take an
advisory lock (an S3 `<key>.lock` object via a conditional put) so concurrent runs
cannot corrupt shared state, with a "locked by whom/since when" error and a
`nivis force-unlock` escape hatch.

Docs impact: modifications only - extends `docs/REMOTE-STATE.md` (the locking
section replaces the "no locking yet (B2)" caveat). No new document; locking is part
of the remote-state story a user already reads there.
