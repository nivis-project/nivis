---
# nixform2-aur6
title: A replacement carries the destroyed object's computed attributes into the new one
status: todo
type: bug
priority: normal
created_at: 2026-09-14T20:38:32Z
updated_at: 2026-10-09T08:23:25Z
---

Replacing `aws_instance.target` in nivis-demos `040_tunnel_target` failed:

```
provider note aws: Deprecated. Attempting to use security_groups within a VPC
  instance. Use vpc_security_group_ids instead.
provider error: creating EC2 Instance: RunInstances,
  api error InvalidGroup.NotFound: The security group
  nivis-tunnel-target-demo does not exist in VPC vpc-0a782a657c7583edc
```

The group did exist, in that exact VPC, under that exact name. EC2 was answering
a question nobody meant to ask: `RunInstances` resolves security groups by NAME
only on the retired EC2-Classic path, so a VPC launch given names fails whether
or not the name is there.

## Where the name came from

The domain sets only `vpc_security_group_ids`. Confirmed by evaluating the IR:

```
vpc_security_group_ids = [ { __ref = aws.aws_security_group.target.id } ]
```

`security_groups` came from `ProposedMsgPack` in `internal/tfvalue/value.go`:

```go
case computed[name]:
    praw, ok := prior[name]   // computed attr absent from config -> prior value
```

The destroyed instance had `security_groups = ["nivis-tunnel-target-demo"]`,
because the AWS provider populates that name list when it reads an instance
back. Nivis carried it into the replacement.

## Why the rule is right and the application is wrong

Carrying prior values for optional-computed attributes is correct for an
UPDATE, and the comment above the function explains what it fixes: without it,
SDKv2 providers see a phantom diff and ForceNew attributes cause a perpetual
`-/+`.

A replacement is not an update. Terraform core plans the new object against a
NULL prior, so optional-computed attributes become unknown rather than
inherited. Nivis plans it against the object it is about to destroy.

## Why it is easy to miss

It only bites when a replacement is planned while the prior object still
exists. Apply again after the destroy half has run and there is no prior left,
so the create path (`EncodeConfig`) is used and it succeeds. That makes it look
like a transient failure.

## Related

nixform2-nwnf, the ordering half of the same weakness: replacements are treated
as a local operation on one node rather than a destroy plus a create with the
graph semantics of each.

## Todo

- [ ] Plan the create half of a replacement against a null prior
- [ ] Test: a resource with an optional-computed attribute the provider fills
      in on read, replaced while the prior object is still in state
- [ ] Check whether the same carry-over affects requires-replace detection


## Verified against OpenTofu (2026-10-09)

OpenTofu does exactly this, and its comment names this bug
(`internal/tofu/node_resource_abstract_instance.go`, `if action.IsReplace()`):

> we want to produce a change object that shows our real prior object but has a
> _new_ object that is built from a null prior object, since we're going to
> delete the one that has all the computed values on it

It re-plans with `PriorState: nullPriorVal` and a `ProposedNewState` rebuilt
as `objchange.ProposedNew(schema, nullPriorVal, config)`. The re-plan happens
in the PLAN walk, not at apply.

## Todo 3 resolves as: no change needed

"Check whether the same carry-over affects requires-replace detection." It
does, and it must keep doing so. `ProposedMsgPack` carrying prior values into
unset computed attributes is what stops SDKv2 providers flagging ForceNew
computed attrs as requires-replace on every plan (the perpetual -/+ the function
was written to kill). The rule is correct everywhere. The defect is that one
plan result serves two callers needing opposite priors.

## What the investigation settled

- The fix belongs in `plan.Plan`, not the driver. `applyOne` already passes
  `pr.PlannedState` and `prior=nil` to the create half, so correcting
  `plan.Plan` fixes apply and report together with NO change in
  `internal/phase`.
- `nivis plan` has the same defect, quieter: its known-after-apply list comes
  from the prior-carrying plan, so a replacement under-reports what will be
  recomputed.
- `client.Plan(Prior: nil)` already produces `ProposedNew(nullPrior, config)`,
  because the backends set `proposed := cfgDV` when prior is nil. No new
  encoding needed.
- `UnknownAfterApply` is display-only (reaches `renderPlan` and nothing
  functional). `NoOp` is guarded by `req.Prior != nil` so a second call
  cannot fabricate one. Nivis carries no provider private state, so nothing is
  lost by planning twice.
- Duplicate diagnostics from the second call are latent only: `plan.Result`
  has no diagnostics field today. Constraint recorded on nixform2-8btx.

## Blocked on nixform2-mbp1

No fake can signal RequiresReplace, so the defect is not reproducible through a
spawned provider. Substrate first.

## OpenSpec

`replace-plans-the-create-half-against-null` (spec-driven; proposal, design,
spec delta on `executor`, tasks). Validated, not implemented.
