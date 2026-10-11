---
# nixform2-hpmb
title: A replacement's destroy is attempted even when resources in state still depend on it
status: completed
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-10-11T10:02:38Z
updated_at: 2026-10-11T11:35:44Z
---

Discovered while exploring nixform2-nwnf on 2026-10-11. This is the cheap guard
half of that bean, separated because it is independent of the ordering fix and
useful before it.

When `applyOne` replaces a resource it destroys the prior object with no check
on what else holds it. The cloud answers instead, late and opaquely. From
nixform2-nwnf:

    error: phase 1: apply "aws.aws_ebs_snapshot_import.bootstrap":
      replace: destroy prior: deleting EBS Snapshot (snap-07765dc0a0d021386):
      api error InvalidSnapshot.InUse: The snapshot is currently in use by
      ami-02f1477c07117d1c5

Nivis already knows enough to say this before calling the provider: the IR graph
gives the resources that reference the one being replaced, and the state store
says which of those exist. The information is on hand and unused.

## What good would look like

Name the dependents and say what to do, rather than relaying an API error about
an id the user never typed. Something a reader can act on: which resources hold
it, that a replacement must release them first, and the escape hatch once one
exists (`createBeforeDestroy`, see the sibling bean).

## Why it survives the ordering fix

A planning fixpoint (nixform2-nwnf) puts FORCE-NEW dependents into the
replacement set, so those get destroyed in the right order. A dependent that
merely holds a reference without being force-new never enters that set, and its
destroy still orphans. The guard is what catches the case the cascade
structurally cannot reach, so it is wanted after that work too, not only before.

## Open design point

Whether the check belongs in `applyOne` beside the `preventDestroy` guard, or in
`plan.Plan` where the operation is decided. The last three changes in this
family all turned on that distinction, and the answer decides whether `nivis
plan` can warn about it too rather than only `nivis apply` failing.


OpenSpec change: `refuse-a-replacement-that-orphans-dependents` (spec-driven;
proposal, design, spec deltas on `executor` and `cli`, tasks). Validated, not
implemented.

Open design point resolved: the check belongs in the driver, not `plan.Plan`.
`plan.Plan` has the client, schema, node, config and prior, and has neither the
IR graph nor the state store. `applyOne` and `PlanReport` have both. One
helper, two callers, so apply and plan cannot disagree.


## Summary of Changes

`DAG.Dependents` is the reverse reachability of `Deps`, in IR order so a
message is stable. `Driver.heldBy` intersects it with the state store, and the
refusal sits beside the `preventDestroy` guard in `applyOne`, before the
destroy. `PlanReport` sets `PlanItem.HeldBy` for a replacement, and
`nivis plan` reports it without failing.

The open design point resolved as recorded: the check needs the IR graph and the
state store together, `plan.Plan` has neither, and `applyOne` and
`PlanReport` have both. One helper, two callers.

### A `__derived` holder is covered for free

`graph.Build` already records a derived leaf as a real ordering dependency
("create after, destroy before") from the refs captured at ingest, so a holder
reached only through a Nix-computed value is not missed. Pinned by
`TestDependentsIncludesDerivedHolders`.

### Verification

The three-layer chain from nixform2-nwnf now has a test; it had none.
Replacing the middle while the top holds it is refused, naming both, with the
middle resource's prior object intact and state unchanged. Proved to
discriminate: with the guard removed the test fails with "expected the
replacement to be refused while top holds middle".

Also covered: a resource nothing depends on still replaces; a dependent declared
in configuration but absent from state does not block (presence in STATE is the
test); and the plan reports the blockage while leaving state untouched.

`go test ./...` green including `tests/e2e`, with
`internal/phase/replace_test.go` and `tests/` unedited and `internal/plan`
and `internal/tfvalue` untouched.

OpenSpec change: `refuse-a-replacement-that-orphans-dependents`.
