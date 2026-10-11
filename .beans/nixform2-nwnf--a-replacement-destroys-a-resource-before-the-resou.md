---
# nixform2-nwnf
title: A replacement destroys a resource before the resources that depend on it
status: todo
type: bug
priority: normal
created_at: 2026-09-14T20:22:58Z
updated_at: 2026-10-11T10:05:43Z
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
