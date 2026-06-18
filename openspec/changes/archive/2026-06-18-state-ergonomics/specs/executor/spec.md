# Spec delta: executor

## MODIFIED Requirements

### Requirement: Lockable local JSON state backend
The executor SHALL persist resource states to a local JSON file with an advisory
lock, supporting get/set/list/delete by resource id. The format is internal (no
tfstate compatibility) and accessed behind an interface that admits future
remote backends. The interface SHALL additionally provide whole-document access:
`Snapshot` returns the entire state as bytes (the canonical state document) and
`Restore` replaces the entire state from bytes (parsing and validating it as a
state document, then writing atomically under the lock). These two operations are
the document-level seam a remote backend implements, so whole-state read/replace
works identically across backends.

Acquiring the advisory lock SHALL NOT block indefinitely: it SHALL time out after
a bounded interval and, on failure, return an actionable error stating that the
state appears locked by another process and naming the lock file, rather than
hanging.

#### Scenario: round-trip a resource state
- WHEN a resource state is set and the store reloaded from disk
- THEN get by id returns the same attributes.

#### Scenario: concurrent writers are serialized
- WHEN two state operations contend
- THEN the advisory lock serializes them and neither write is lost.

#### Scenario: partial state survives a crash mid-run
- GIVEN resource A's state has been set and persisted
- WHEN the process exits before B is applied
- THEN reopening the store still returns A's state.

#### Scenario: snapshot round-trips through restore
- GIVEN a store with several resource states
- WHEN its `Snapshot` bytes are passed to `Restore` on an empty store
- THEN that store then lists exactly the same resource states.

#### Scenario: restore replaces the whole document
- GIVEN a store containing resource X
- WHEN `Restore` is called with a snapshot that contains only resource Y
- THEN the store afterwards contains Y and not X.

#### Scenario: restore rejects a malformed document
- WHEN `Restore` is called with bytes that are not a valid state document
- THEN it returns an error and the existing state is left unchanged.

#### Scenario: a contended lock fails with an actionable error, not a hang
- GIVEN another process holds the state lock
- WHEN a state operation cannot acquire the lock within the timeout
- THEN it returns an error naming the lock file and saying the state appears locked, rather than blocking forever.
