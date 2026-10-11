---
# nixform2-hpmb
title: A replacement's destroy is attempted even when resources in state still depend on it
status: todo
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-10-11T10:02:38Z
updated_at: 2026-10-11T10:05:43Z
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
