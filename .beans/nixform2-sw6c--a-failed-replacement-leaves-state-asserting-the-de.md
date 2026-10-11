---
# nixform2-sw6c
title: A failed replacement leaves state asserting the destroyed resource still exists
status: completed
type: bug
priority: high
tags:
    - discovered
created_at: 2026-10-11T06:56:59Z
updated_at: 2026-10-11T10:29:34Z
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


## Summary of Changes

`applyOne` now deletes the resource's state entry as soon as the destroy of the
prior object returns, before attempting the create. `internal/destroy` already
did this per resource; the replacement path was the only place in the executor
that removed a cloud object without removing its entry. `preventDestroy` still
refuses first, so the delete cannot run for a protected resource.

The fakes gained deterministic opt-in failure injection
(`TERRAE_NIVIS_FAKE_FAIL="apply:<typename>"`), without which this executor
failure path could not be reached through a spawned provider at all.

### Three things that were wrong on the first attempt

1. The seam checked at the top of `ApplyResourceChange`. In the protocol a
   DESTROY is `ApplyResourceChange` with a null planned state, so it failed the
   destroy rather than the create, and the run errored before anything was
   destroyed. Moved after the destroy branch in all three fakes.
2. The recovery test re-ran with the NEW config. A stale entry still plans as a
   replacement there, so the object is recreated either way and the test passed
   with the fix reverted. Rewritten to model the real harm: the user REVERTS the
   configuration, a stale entry matches it exactly, and the run reports no
   change while the object does not exist.
3. That rewrite asserted on `res.Applied`, which the phase loop appends to for
   every node it resolves, no-ops included. Rewritten to assert the operation is
   a create.

Only after all three did reverting the fix turn the tests red:
`the resource resolved as 3 (OpNoop), want a create`.

### Verification

Both e2e tests proved to discriminate. `go test ./...` green including
`tests/e2e`. The only non-test executor change is 8 lines in `driver.go`;
`cmd/` is unedited and `internal/destroy` is untouched.

The tests drive the real `provider-delta` binary and a real on-disk state
store. The IR comes from `phase.StubEvaluator` rather than the Nix topology,
because no flake attribute declares a force-new attribute and inventing a public
one for a failure-path test would be a larger change than the fix.

OpenSpec change: `replace-clears-state-before-creating`.
