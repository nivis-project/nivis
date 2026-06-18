# Proposal: state-ergonomics

## Why
State has `list`/`show`/`rm` and a configurable path, but two gaps remain for a
daily-driver (A6 of "Road to v1", beans-z8e1): there is no way to read or replace
the **whole** state document (a `pull`/`push`), and a held lock makes the CLI
**hang forever** with no message. The whole-document accessor is also the seam
Phase B's remote backend (B1) reuses, so getting its shape right now de-risks
that milestone.

## What changes
- **`Store` gains `Snapshot()`/`Restore()`** (the document-level seam): `Snapshot`
  returns the entire state as bytes (the canonical JSON document), `Restore`
  replaces the entire state from bytes. The per-resource `Get`/`Set`/`Delete`/`List`
  stay. `fileStore` implements them; **Phase B's S3 backend will implement the same
  two methods**, so `state pull`/`push` work identically against any backend.
- **`nivis state pull`** writes the snapshot to stdout (or `--out <file>`), and
  **`nivis state push`** replaces state from stdin (or `--in <file>`). `push`
  validates the input parses as a Nivis state document before writing.
- **`push` is guarded:** it reports how many resources the input has and the
  current state has, and requires confirmation before overwriting; `--force`
  (a.k.a. `--yes`) skips the prompt, and is **required** when stdin is not a TTY
  (non-interactive), so a scripted `push` is always explicit.
- **Lock acquisition times out.** `withLock` tries the advisory lock with a short
  timeout instead of blocking forever; on failure it returns an actionable error
  naming the lock file and that another process likely holds it (so a stale or
  contended lock is a clear message, not a hang). A `--lock-timeout` may tune it;
  default a few seconds.
- **`list`/`show`/`rm` polish:** `show` of a missing id and `rm` of a missing id
  give clear messages (today `rm` of a missing id silently succeeds); `list` notes
  when state is empty.

This is state-handling DX plus one interface addition; no IR change, no change to
how resources are planned or applied.

## Decisions (settled with the maintainer)
- **`Snapshot`/`Restore` on the `Store` interface**, not a CLI-only path, so Phase
  B reuses the seam.
- **`push` confirms unless `--force`**, required non-interactively (it overwrites
  the state of record).
- **Lock times out with a clear error**, rather than blocking indefinitely.

## Non-goals
- The remote (S3) backend itself (Phase B / B1). This only defines and implements
  the seam locally.
- `state mv` (rename an id) or surgical single-resource import; out of scope.
- A force-unlock command (pairs with the remote lock in B2); for the local file a
  stale lock can be removed by hand, which the error message points at.
- tfstate-format compatibility (DESIGN: state stays Nivis's own).

## Impact
- `internal/state`: `Store` gains `Snapshot()`/`Restore()`; `fileStore` implements
  them (snapshot = the marshaled document; restore = parse + atomic write under
  lock). `withLock` gets a timeout + an actionable lock error.
- `cmd/nivis`: `state pull`/`state push` subcommands (`--out`/`--in`, `--force`),
  the lock-timeout flag, and the `show`/`rm` message polish.
- Tests: `internal/state` (snapshot round-trips through restore; restore replaces;
  restore rejects malformed input; lock timeout returns the actionable error);
  `cmd/nivis` (push refuses non-interactively without `--force`; push with --force
  restores).

Docs impact: new paragraph/section; a "Move state around (pull/push)" note plus a
lock-error note in docs/INSTALL.md or a short state section. No new document: this
is more capability on the already-documented state command, not a new concept
(per docs/DOCS-GATE.md).
