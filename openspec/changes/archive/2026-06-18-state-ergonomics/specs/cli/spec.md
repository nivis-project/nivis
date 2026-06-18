# Spec delta: cli

## ADDED Requirements

### Requirement: Move state with pull and push, guarded against accidental loss
The CLI SHALL provide `state pull` and `state push`. `state pull` SHALL write the
whole state document (the backend's snapshot) to stdout, or to a file with
`--out`. `state push` SHALL replace the whole state from stdin, or from a file
with `--in`, after the input parses as a valid state document. Because `push`
overwrites the state of record, it SHALL require confirmation before replacing
existing state: it SHALL report the resource counts (incoming vs current) and
prompt, and SHALL proceed without prompting only when `--force` (equivalently
`--yes`) is given. When the input is not an interactive terminal (a pipe or
redirect), `push` SHALL require `--force` and SHALL refuse otherwise with a clear
message, so a scripted push is always explicit.

#### Scenario: pull writes the snapshot
- GIVEN state with several resources
- WHEN `nivis state pull` runs
- THEN the whole state document is written to stdout (or the `--out` file), and re-pushing it reproduces the same state.

#### Scenario: push replaces state after confirmation
- GIVEN a valid state document on stdin and `--force`
- WHEN `nivis state push` runs
- THEN the state is replaced by the document's resources.

#### Scenario: a non-interactive push without --force is refused
- GIVEN `state push` reading from a pipe (non-TTY) without `--force`
- WHEN it runs
- THEN it refuses with a message telling the user to pass `--force`, and state is unchanged.

#### Scenario: push rejects malformed input
- GIVEN `state push` given input that is not a valid state document
- WHEN it runs
- THEN it fails with an error and leaves the existing state unchanged.

### Requirement: State commands give clear messages on missing ids and empty state
`state show` and `state rm` SHALL give a clear message when the named id is not in
state (rather than failing obscurely or silently succeeding), and `state list`
SHALL indicate when state is empty.

#### Scenario: rm of a missing id is reported
- GIVEN an id not present in state
- WHEN `nivis state rm <id>` runs
- THEN it reports that the id was not in state (not a silent success).

#### Scenario: list of empty state says so
- GIVEN empty state
- WHEN `nivis state list` runs
- THEN it indicates there are no resources in state.
