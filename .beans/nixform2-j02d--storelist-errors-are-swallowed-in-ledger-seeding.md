---
# nixform2-j02d
title: Store.List() errors are swallowed in ledger seeding
status: todo
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-09-07T16:41:11Z
updated_at: 2026-09-07T20:03:36Z
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
