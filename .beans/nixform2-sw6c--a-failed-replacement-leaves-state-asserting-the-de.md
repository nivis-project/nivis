---
# nixform2-sw6c
title: A failed replacement leaves state asserting the destroyed resource still exists
status: todo
type: bug
priority: high
tags:
    - discovered
created_at: 2026-10-11T06:56:59Z
updated_at: 2026-10-11T06:56:59Z
---

Discovered while exploring nixform2-nwnf on 2026-10-11.

`Store.Delete` is called in exactly one place in the executor,
`internal/destroy/destroy.go:83`. The replacement path in
`internal/phase/driver.go` destroys the prior object and relies on the create
half overwriting the state entry:

    client.Destroy(prior)          the cloud object is gone
    ---- failure window ----       state still says it exists
    apply.Apply(...) -> store.Set

If the create fails, or the run is interrupted, state is left asserting an
object that is not there.

## Why it is not merely cosmetic

Whether it heals depends on a flag chosen for unrelated reasons:

- refresh on (the default): the next run reads the resource, gets nothing,
  treats it as an out-of-band deletion and plans a create. It heals.
- `--refresh=false`: the stale attributes are returned as prior state
  (`internal/phase/driver.go`, `priorState` returns `stored` when `NoRefresh`),
  so the next plan is made against an object that does not exist.

A failed apply is ordinary: a provider rejecting the new config, an expired
credential, an interrupted run. `internal/destroy` already does this correctly,
per resource. The replacement path is the only place that removes a cloud object
without removing its entry.

## Why now, ahead of nixform2-nwnf

The ordering work explored for nwnf adds a destroy pass that removes several
resources before creating any of them, widening this window from one resource to
a whole cascade. The invariant should hold for one resource before the window is
widened.

## Untestable today

No fake can fail an operation, so this executor failure path cannot be reached
through a spawned provider. The change adds deterministic opt-in failure
injection to the fakes, which is what makes the e2e test possible.

## OpenSpec

`replace-clears-state-before-creating` (spec-driven; proposal, design, spec
deltas on `executor` and `fake-providers`, tasks). Validated, not implemented.
