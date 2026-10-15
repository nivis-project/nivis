---
# nixform2-j02d
title: Store.List() errors are swallowed in ledger seeding
status: completed
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-09-07T16:41:11Z
updated_at: 2026-10-15T19:33:02Z
parent: nixform2-kovh
---

Found while implementing `nixform2-ebx5` (state migration between backends), OpenSpec change `state-migrate-between-backends`.

`internal/phase/driver.go` seeds the ledger from stored state in two places and **discards the error**:

- `driver.go:218` (`PlanReport`): `if stored, err := d.Store.List(); err == nil { ... }`
- `driver.go:339` (`ResolveOutputs`): the same pattern.

A backend read failure (unreachable location, denied credentials, a transient S3 error) therefore degrades silently to "no stored outputs" instead of surfacing. `Store.Get()` errors *do* propagate, so `plan`/`apply` still fail loudly — but `nivis output` goes through `ResolveOutputs`, so it can report unresolved outputs (or an eval error) instead of naming the real backend failure.

## Why this matters

Silently treating "I cannot read your state" as "there is no state" is the same class of bug the missing-bucket decision in `state-migrate-between-backends` exists to prevent (see that change's design.md, Decision 6). The seeding path is the last place it still happens.

## Acceptance criteria

- Both call sites propagate the backend read error (or deliberately tolerate only a documented, narrow "no state yet" condition).
- A test covers `nivis output` against an unreadable backend reporting the backend error, not an eval/unresolved-output error.


## Summary of Changes

All three whole-document state reads now propagate. The bean names two; there
were three.

- `PlanReport` and `ResolveOutputs` return the backend error instead of
  seeding an empty ledger, so `nivis plan` no longer reports every resource as
  a create and `nivis output` no longer reports unresolved outputs when the
  real problem is that state is unreadable.
- `heldBy` returns an error, and the replacement guard FAILS CLOSED: unable to
  determine what depends on a resource, the executor refuses to destroy it.

### The third site was mine, from the day before

`heldBy` was added in `refuse-a-replacement-that-orphans-dependents`
(commit eda6c55). It is the worst of the three because it guards a destructive
step: unable to read state it concluded nothing depended on the resource and let
the replacement proceed, removing the guard at exactly the moment it mattered.

The comment there justified swallowing the graph-build error because the caller
was past the point where the graph was validated. That argues from the
LIKELIHOOD of the failure rather than the COST of being wrong about it, which is
the same mistake as the two older sites. Removed.

### Why this was propagation and not a redesign

`fileStore.read` (`internal/state/state.go:117`) already returns an empty
document and no error when the file does not exist. So "no state yet" never
reached these sites as an error: they were not tolerating a legitimate case,
they were losing everything. Acceptance criterion 1 is met without the
"documented, narrow condition" escape it allowed for.

### Verification

Proved to discriminate. With the propagation reverted, `PlanReport` returns 3
items while the backend is unreadable (all of which would report as creates) and
the replacement is not refused. Four tests: both read paths surface the error,
the replacement is refused with the prior object intact, and a first run with no
state document still reports creates, which is the regression this change could
plausibly have caused.

`go test ./...` green including `tests/e2e`, with `internal/state` and
`tests/` untouched.

OpenSpec change: `surface-state-read-failures`.
