# Spec delta: executor

## ADDED Requirements

### Requirement: S3 remote state backend
The executor SHALL provide an S3-backed implementation of the `Store` interface,
selected when the IR declares `backend.type == "s3"`. It SHALL store the entire
state **document** (the same canonical JSON `Snapshot`/`Restore` bytes the local
store uses, Nivis's own format, no tfstate compatibility) as a single S3 object at
the configured `bucket`/`key`, and SHALL implement per-resource `Get`/`Set`/
`Delete`/`List` as read-modify-write of that object. Every write SHALL request
server-side encryption. Credentials SHALL come from the AWS default credential
chain (environment, shared profile, instance role), NEVER from the IR `backend`;
only the location (`bucket`, `key`, `region`, and an optional `endpoint` override)
comes from `backend`. A missing object SHALL read as an empty state document (a
fresh stack), not an error.

#### Scenario: round-trip a resource state through S3
- GIVEN an s3-backed store
- WHEN a resource state is set and the store is reopened against the same bucket/key
- THEN get by id returns the same attributes (the document was persisted to the S3 object).

#### Scenario: a fresh (missing) object reads as empty
- GIVEN an s3-backed store whose object does not yet exist
- WHEN the state is listed
- THEN it returns empty with no error, and the first write creates the object.

#### Scenario: writes are server-side encrypted
- GIVEN an s3-backed store
- WHEN it writes the state object
- THEN the PutObject request specifies server-side encryption.

#### Scenario: snapshot/restore round-trip through S3
- GIVEN an s3-backed store with several resource states
- WHEN its `Snapshot` bytes are passed to `Restore` on a store over a different key
- THEN that store then lists exactly the same resource states.

#### Scenario: credentials are not taken from config
- GIVEN an IR `backend` for s3
- WHEN the store is opened
- THEN the bucket/key/region come from `backend` and the credentials come from the AWS default chain, and no credential field in `backend` is read.

### Requirement: State backend selection from the IR
The executor SHALL choose the state backend from the IR's optional `backend`
block: `type == "s3"` selects the S3 store; an absent `backend` (or a local type)
uses the local file store (today's default and the `--state` path). Selection SHALL
validate the backend's required keys (for s3: `bucket`, `key`, `region`) and fail
with an actionable error when a required key is missing or the `type` is
unsupported. All commands that use state (plan, apply, destroy, refresh, the state
subcommands) SHALL operate through the selected `Store` without other changes,
since they depend only on the interface.

#### Scenario: s3 backend selects the S3 store
- GIVEN an IR declaring `backend = { type = "s3"; bucket; key; region }`
- WHEN a state-using command runs
- THEN it operates against the S3 store, not the local file.

#### Scenario: no backend uses the local file store
- GIVEN an IR with no `backend`
- WHEN a state-using command runs
- THEN it uses the local file store at the `--state` path (unchanged behaviour).

#### Scenario: an unsupported backend type errors clearly
- GIVEN an IR whose `backend.type` is not a supported backend
- WHEN the store is opened
- THEN it fails with an actionable error naming the unsupported type.

#### Scenario: a missing required s3 key errors clearly
- GIVEN an s3 `backend` missing `bucket` (or `key`/`region`)
- WHEN the store is opened
- THEN it fails with an actionable error naming the missing key.
