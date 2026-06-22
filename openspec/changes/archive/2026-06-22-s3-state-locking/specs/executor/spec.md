# Spec delta: executor

## ADDED Requirements

### Requirement: Advisory state locking for the S3 backend
The S3 backend SHALL provide an advisory lock so two concurrent mutating runs
cannot corrupt shared state. The lock SHALL be a sibling S3 object at `<key>.lock`
created with a conditional put (`IfNoneMatch: "*"`, atomic create-if-absent): a
successful create acquires the lock; a precondition failure means the lock is held.
The lock object SHALL contain lock information identifying the holder (a user, a
host, a pid, an ISO-8601 timestamp, the operation, and a generated lock id).
Acquiring a held lock SHALL fail with an actionable error that reads the holder's
information back (who, since when, which operation) and points at the force-unlock
escape hatch. Releasing SHALL delete the lock object; an `Unlock` that is given a
lock id SHALL refuse to delete a lock whose id does not match (so a stale release
cannot drop another run's lock), while a forced unlock SHALL delete the lock
unconditionally. The lock seam SHALL be an OPTIONAL interface a backend MAY
implement, so the `Store` interface is unchanged and a backend without locking
(e.g. the local file store today) simply does not lock.

#### Scenario: acquiring a free lock succeeds
- GIVEN an s3 backend with no lock object
- WHEN the lock is acquired
- THEN the `<key>.lock` object is created with the holder's lock information and the acquire succeeds.

#### Scenario: a held lock blocks a second acquire with the holder's info
- GIVEN an s3 backend whose lock is already held by run A
- WHEN run B tries to acquire it
- THEN the acquire fails with an error naming run A's holder (user/host), the time since when it has been held, and the operation, and pointing at force-unlock.

#### Scenario: unlock releases the lock
- GIVEN a lock held by a run
- WHEN that run unlocks with its own lock id
- THEN the lock object is removed and a subsequent acquire succeeds.

#### Scenario: unlock with a mismatched id is refused
- GIVEN a lock held with id X
- WHEN unlock is called with id Y (≠ X)
- THEN it refuses and does not delete the lock (it is not this caller's lock).

#### Scenario: force-unlock clears a held lock
- GIVEN a lock held (e.g. left by a crashed run)
- WHEN force-unlock runs
- THEN the lock object is deleted unconditionally and a subsequent acquire succeeds.

#### Scenario: a backend without locking does not lock
- GIVEN a store that does not implement the lock seam (the local file store)
- WHEN a mutating run uses it
- THEN no lock is taken and behaviour is unchanged.
