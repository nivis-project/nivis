---
# nixform2-z8e1
title: 'A6: State ergonomics'
status: completed
type: epic
priority: normal
tags:
    - roadmap
created_at: 2026-06-16T13:37:59Z
updated_at: 2026-06-18T08:14:49Z
parent: nixform2-zdj0
---

The small state-handling things a real project needs day to day, and the seam Phase B's remote backend will reuse.

ROADMAP Phase A6. GROUND TRUTH: configurable --state path and `state list/show/rm` exist; the Store interface is the remote-backend seam.

## Scope
- Polish `state list/show/rm`.
- A `state pull/push` shape that the Phase B remote backend (B1) will reuse.
- Clear errors on stale or locked state files.


---
## Summary of Changes
DONE via OpenSpec change state-ergonomics (archived 2026-06-18-state-ergonomics):

- SEAM: state.Store gains Snapshot() ([]byte) and Restore([]byte) (the document-level seam Phase B's S3 backend reuses). fileStore: Snapshot = the marshaled document under lock; Restore = parse+validate (key==id invariant) BEFORE touching the file, then atomic write under lock, so malformed input leaves state unchanged.
- CLI: `nivis state pull` (whole document -> stdout or --out) and `nivis state push` (replace from stdin or --in). push reports incoming-vs-current counts and CONFIRMS before overwriting; --force/--yes skips; --force is REQUIRED non-interactively (refuses a piped push otherwise) so a scripted push is explicit. Validates input before writing.
- LOCK: withLock no longer blocks forever; it retries LOCK_EX|LOCK_NB until a timeout (default 5s, OpenWithLockTimeout to tune) then fails with an actionable error naming the lock file ("state appears locked by another nivis process ..."). 
- POLISH: state list says "No resources in state." when empty; state rm of a missing id reports "not in state" instead of silently succeeding; list/rm write via cmd.OutOrStdout().

DECISIONS (with maintainer): Snapshot/Restore on the Store interface (not CLI-only) so Phase B reuses it; push confirms unless --force (required non-interactive); lock times out with a clear error.

VERIFIED live: pull -> push --force round-trips a document; list confirms; a non-interactive push without --force is refused. Tests: state (snapshot round-trips through restore; restore replaces; restore rejects malformed leaving state intact; held lock -> actionable timeout error via OpenWithLockTimeout(200ms)); cmd/nivis (push refuses non-interactive w/o --force, push --force restores, push rejects malformed). Full gate green: gofmt, go build, go test, check-docs-ssot (incl docs-coverage + mdbook).

DOCS (new section): docs/INSTALL.md "Working with state" (list/show/rm, pull/push with the --force/non-interactive rule, and the lock-error note).

NON-GOALS (deferred): the S3 remote backend itself (B1), state mv, force-unlock command (pairs with B2), tfstate compat.
