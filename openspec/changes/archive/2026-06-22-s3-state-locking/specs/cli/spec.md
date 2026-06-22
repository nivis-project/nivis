# Spec delta: cli

## ADDED Requirements

### Requirement: Mutating commands take the state lock; force-unlock clears it
The CLI SHALL acquire the state lock around mutating runs (`apply` and `destroy`)
when the selected backend supports locking, holding it for the duration of the run
and releasing it when the run finishes (including on failure). It SHALL report
acquiring and releasing the lock. Read-only commands (`plan`, `refresh`, `output`,
`state pull`) SHALL NOT take the lock. When the lock is already held, a mutating
command SHALL fail before doing any work, with the holder error (who/since/
operation) and a pointer to force-unlock. The CLI SHALL provide a `force-unlock`
command that clears a stuck lock (e.g. after a crashed run), confirming first in an
interactive session and accepting a non-interactive override flag, and reporting
what was unlocked. A backend that does not support locking (the local file store)
SHALL make `force-unlock` a clear no-op-or-error rather than a crash.

#### Scenario: apply takes and releases the lock
- GIVEN an s3 backend with no lock held
- WHEN `nivis apply` runs to completion
- THEN it acquires the lock before applying, releases it after, and reports both.

#### Scenario: apply is blocked while the lock is held
- GIVEN an s3 backend whose lock is held by another run
- WHEN `nivis apply` runs
- THEN it fails before applying anything, naming the holder and the time since held, and suggesting force-unlock.

#### Scenario: a read-only command does not lock
- GIVEN an s3 backend
- WHEN `nivis plan` (or `refresh`/`output`/`state pull`) runs
- THEN it does not acquire the state lock.

#### Scenario: force-unlock clears a stuck lock
- GIVEN an s3 backend with a stuck lock from a crashed run
- WHEN `nivis force-unlock` runs (confirmed, or with the non-interactive override)
- THEN the lock is cleared and a subsequent `apply` can acquire it, and the command reports what it unlocked.

#### Scenario: the lock release survives a failed run
- GIVEN an s3 backend and an `apply` that errors mid-run
- WHEN the run fails
- THEN the lock is still released (not left held), so the next run is not blocked by this one.
