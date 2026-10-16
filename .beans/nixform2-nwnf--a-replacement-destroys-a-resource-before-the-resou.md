---
# nixform2-nwnf
title: A replacement destroys a resource before the resources that depend on it
status: completed
type: bug
priority: normal
created_at: 2026-09-14T20:22:58Z
updated_at: 2026-10-16T10:38:34Z
---

Applying `040_tunnel_target` in nivis-demos, where a changed image forces the
snapshot to be replaced, nivis destroyed the snapshot first and AWS refused:

```
error: phase 1: apply "aws.aws_ebs_snapshot_import.bootstrap":
  replace "aws.aws_ebs_snapshot_import.bootstrap": destroy prior:
  deleting EBS Snapshot (snap-07765dc0a0d021386):
  api error InvalidSnapshot.InUse: The snapshot snap-07765dc0a0d021386
  is currently in use by ami-02f1477c07117d1c5
```

## The graph

```
aws_s3_object.image
        |
aws_ebs_snapshot_import.bootstrap      <- must be replaced
        |
aws_ami.bootstrap                      <- still holds the old snapshot
        |
aws_instance.target
```

A replacement has a destroy half, and that half belongs in the reverse of
creation order: instance, then AMI, then snapshot. Nivis ran the snapshot
destroy while both dependents were still in place.

OpenTofu gets this right by sorting destroy nodes against the reversed
dependency graph. Nivis appears to treat a replacement as a local operation on
one node.

## Why it matters beyond this domain

This is the shape of every image-based domain: an artefact, a cloud-side
conversion of it, a machine that boots the result. Three layers is the minimum,
so any nivis user who changes an image hits this. The workaround is manual
(`aws ec2 deregister-image`, then apply again), which means a changed image
cannot be rolled out unattended.

## Repro

nivis-demos, `stack/040_tunnel_target/domain.nix`, after commit 5749514 made the
S3 key carry the image store hash. Change the image, apply, and the snapshot
replacement fails on the first attempt.

## Todo

- [ ] Decide whether the destroy half of a replacement is ordered by the
      reversed graph, or whether replacements gain a create-before-destroy mode
- [ ] Order destroys against the reversed dependency graph
- [ ] Test: a three-layer chain where the middle node is replaced


## Research findings (2026-10-11)

Explored, with the pieces verified against upstream source rather than memory.

### The premise needs correcting

"OpenTofu gets this right by sorting destroy nodes against the reversed
dependency graph" is half the story. Reverse ordering only helps when the
dependents are ALSO in the destroy set. OpenTofu gets them there through
plan-time unknown propagation: a replaced resource's id becomes unknown, which
makes a dependent's force-new attribute unknown, which replaces the dependent
too. Reverse-ordering a single-element destroy set does nothing.

### Both toolkits do propagate replacement through an unknown

Verified in source:

- terraform-plugin-framework,
  `resource/schema/stringplanmodifier/requires_replace_if.go`: replaces when
  `!PlanValue.Equal(StateValue)`, and an unknown plan value is not equal to a
  known state value.
- terraform-plugin-sdk, `helper/schema/schema.go`:
  `d.RequiresNew = d.Old != d.New || d.NewComputed`, with the comment
  "NewComputed - This field is being computed, hence a potential change in
  value, mark as causing a new resource."

So a cascade built on unknown propagation is sound for both halves of the
ecosystem.

### The cascade is computable without an IR change

A planning fixpoint, seeded from state:

    ledger := stored outputs
    repeat:
        IR := nix eval (ledger)
        plan every node against its prior
        R  := nodes whose plan says replace
        if R did not grow: stop
        ledger := ledger minus R's outputs

Dropping a replaced resource from the ledger makes dependents' `__ref` leaves
unresolved AND makes Nix re-emit `__derived` leaves for anything computed from
it (`nix/lib/ref.nix`: `derived` resolves only once every input is known;
`toIR`'s `cleanLeaf` emits `{ __derived = { inputs } }` otherwise). So the
provenance that evaluation erased is restored by re-evaluation. Terminates: R
only grows, bounded by node count.

Cost: one `nix eval` plus one plan per node per round, rounds equal to cascade
depth. Eval count is something nixform2-01kd already spent a change optimizing.

### Strategy

1. nixform2-cjxz: the fake's requires-replace comparison is wrong (substrate).
2. nixform2-sw6c: a failed replacement leaves state lying. Fixed ahead of this,
   because the destroy pass below widens that window to a cascade.
3. This bean: planning fixpoint, then reverse-ordered destroy pass.
4. `createBeforeDestroy`: C cannot help a dependent that merely holds a
   reference without being force-new. That is Terraform's own answer for the
   case reverse ordering does not reach, so it is wanted beside C, not instead.

Also worth deciding deliberately: C moves the replacement decision from
per-resource during apply to a set computed up front against stored state. That
is closer to Terraform and introduces ordinary plan/apply skew, which nivis has
so far avoided by never planning ahead.


## Split into beans (2026-10-11)

- nixform2-cjxz: substrate, the fake's requires-replace comparison. Proposed.
- nixform2-sw6c: a failed replacement leaves state lying. Proposed, high.
- nixform2-hpmb: refuse a replacement that would orphan dependents. Proposed.
- nixform2-vh2s: `createBeforeDestroy`, for the case a cascade cannot reach.
- This bean: the planning fixpoint and the reverse-ordered destroy pass. Not yet
  proposed; it depends on nixform2-cjxz landing, and carries one decision worth
  taking deliberately (it moves the replacement decision from per-resource
  during apply to a set computed up front, introducing ordinary plan/apply skew
  that nivis has so far avoided).

## OpenSpec change (2026-10-16)

`cascade-a-replacement-through-dependents` (store: nivis). Proposed, not yet
implemented.

Scope: a replace-set prologue per phase, destroys in reverse dependency order,
the refusal narrowed to holders the provider does not replace, the plan
reporting the cascade, and `docs/REPLACEMENT.md`.

Two things the exploration changed against what this bean assumed:

- The fixpoint needs NO `nix eval`. `graph.ResolveTFTF` re-resolves TF->TF
  references in process and `__ref` already encodes as unknown, so a round costs
  provider plan calls only. The bean's per-round eval is the general case
  including Nix-computed values, which design D4 puts out of scope by refusing
  them.
- Every primitive already exists: `DAG.DestroyOrder`, `DAG.Dependents`,
  `ResolveTFTF`, `tfvalue.isUnresolved`. What is missing is the prologue that
  wires them.

The plan/apply skew decision this bean flagged is taken in design D7: accepted,
bounded to one phase of one run, no persisted plan.

The prologue runs once per phase, not once per run (D3). Not a spike question:
the driver already assumes a node's config is stable once applied
(`applied[id] = true`, never revisited), so the prologue inherits that rather
than introducing it, and per-phase is the only version that reaches a resource
revealed in a later phase.

Task 1 is a spike against the real postgres provider and may end the change: the
premise that an unknown in a force-new attribute returns requires-replace is
verified in upstream source and has never been executed, which is the exact
shape of nixform2-1mk0, aur6, msl4 and cjxz.

Also recorded during exploration: hpmb already turned this from a corruption bug
into a capability gap. Nivis refuses cleanly before destroying anything, so the
remaining cost is that a changed image cannot be rolled out unattended, not that
a run damages infrastructure.

## Spike result (2026-10-16): the premise holds

Measured against cyrilgdn/postgresql 1.26.0, a real unmodified provider binary,
driven through `internal/plugin` with a local postgresql 17.10. Not against a
fake, deliberately.

| probe                                          | RequiresReplace |
|------------------------------------------------|-----------------|
| `database.lc_collate` C to POSIX                | true            |
| control: schema unchanged                       | false (NoOp)    |
| `schema.database` = UNKNOWN                     | **true**        |
| `schema.owner` = UNKNOWN (not identity)         | false (NoOp)    |
| `database.owner` = UNKNOWN (not identity)       | false (NoOp)    |
| `role` renamed                                  | false           |

Row 3 is the premise the whole design rests on: a provider turns an unknown in
an identity-bearing attribute into a requires-replace. Rows 4 and 5 are what
stop the cascade over-reaching, and they matter as much: an unknown in an
attribute the provider can update in place does NOT force a replacement, so
asking the provider rather than assuming (design D1) is load-bearing, not
ceremony.

Row 6 is a fixture constraint discovered here: `postgresql_role` supports
ALTER ROLE RENAME, so a role is never replaced and cannot be the head of a
cascade in the real-provider test. The chain used instead is
`postgresql_database` -> `postgresql_schema`, where `schema.database` is
identity-bearing.

## Summary of Changes

Shipped as OpenSpec change `cascade-a-replacement-through-dependents`.

A replacement now carries through to the resources that hold the replaced
object. Before each phase's apply loop, the executor computes the set of
resources that must be replaced together, refuses if anything in state holds a
member without joining it, and otherwise destroys the set in reverse dependency
order so nothing is destroyed while something holds it. The forward loop then
recreates the members through the path it already had; it was not modified.
`nivis plan` reports the whole set before the run.

The refusal narrowed rather than disappeared. A holder the provider plans as an
in-place update still blocks, and the message now names create-before-destroy
(`nixform2-vh2s`) as what that case needs.

### What the spike changed

The premise, that a provider turns an unknown in an identity-bearing attribute
into a requires-replace, was verified in upstream source and never executed.
Measured against cyrilgdn/postgresql 1.26.0 it holds, and the negative controls
hold too: an unknown in `owner` is an in-place update. That is why the design
asks the provider instead of assuming every holder must go.

### The design error the real provider found

The design said the fixpoint needed no `nix eval`, because `ResolveTFTF` can
re-resolve references in process. That is wrong. Nivis evaluates with the
recorded outputs injected, and an evaluation that can see a value bakes it in:
the reference leaf AND the IR edge recording it are both gone from the graph the
executor receives. Dropping the target and evaluating again is the only way the
dependency comes back, which is what this bean proposed originally.

It surfaced as `nivis plan` reporting a cascade as "no change" while apply
performed it, because `Run` starts with an empty ledger and `PlanReport` seeds
from state before evaluating. Every Go test passed throughout, because the stub
evaluator returns the same IR whatever the ledger holds. The project's recurring
failure shape, in a new place: a fake that agrees with our reading.

Fixed by re-evaluating per fixpoint round, with round one still evaluation-free
so a run that replaces nothing pays nothing. The regression
(`TestPlanReportsACascadeThroughABakedReference`) uses a stub that bakes the way
the real evaluator does.

### Also found

- `postgresql_role` supports ALTER ROLE RENAME, so a role is never replaced.
- glibc treats `POSIX` as an alias for `C`, so a database created with
  `lc_collate = "POSIX"` stores `C` and would make nivis propose the same
  replacement forever. The fixture uses `C` and `C.utf8`.
- `postgresql_schema.if_not_exists` is a create-time flag a read cannot return,
  so importing a schema needs it seeded. A non-credential reason to seed, now
  in `docs/IMPORT.md`.

### Not done here

`nixform2-vh2s` (create-before-destroy) is the remaining case: a holder the
provider updates in place. A cascade cannot reach it, and the refusal now says
so by name.
