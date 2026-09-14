---
# nixform2-aur6
title: A replacement carries the destroyed object's computed attributes into the new one
status: todo
type: bug
created_at: 2026-09-14T20:38:32Z
updated_at: 2026-09-14T20:38:32Z
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
