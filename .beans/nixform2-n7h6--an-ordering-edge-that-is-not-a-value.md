---
# nixform2-n7h6
title: An ordering edge that is not a value
status: todo
type: feature
created_at: 2026-09-14T21:25:40Z
updated_at: 2026-09-14T21:25:40Z
---

Nivis derives every dependency from references inside a resource config. That
covers the common case and covers it well: an edge that comes from data flow
cannot go stale and documents itself. It leaves one case with no expression at
all, and that case is ordinary in infrastructure.

## The case

A must exist before B, and B needs nothing from A.

Found in nivis-demos `040_tunnel_target`, which is also where `020` has it:

```
aws_iam_role.vmimport ────┬──> aws_iam_role_policy_attachment.vmimport
aws_iam_policy.vmimport ──┘

aws_iam_role.vmimport ────────> aws_ebs_snapshot_import.bootstrap
aws_s3_bucket.image ──────────>
```

The import assumes the role, and the role can only read the bucket once the
policy is attached. The import references the role; the attachment references
the role and the policy; neither references the other. So they land in the same
phase as siblings:

```
Phase 2  3 nodes
  = aws.aws_iam_role_policy_attachment.vmimport  213ms
  = aws.aws_s3_object.image  143ms
  -/+ aws.aws_ebs_snapshot_import.bootstrap  7m22s
```

Nothing orders them. It has not bitten because an attachment finishes in 213ms
and an import is slow to start, and because IAM happened to be consistent in
time. That is luck, not a guarantee.

## What authors do instead

In the same domain, an activation resource needs to run after the instance and
consumes nothing from it. The workaround:

```nix
stream_id = derived {
  inputs = [ (instance.refAttr "id") ];
  render = _: vars.tunnelStreamId;      # the input is discarded
};
```

It works. It costs three things:

1. The IR states that a value derives from a resource when it does not. The IR
   is the only place a reader can find the truth about a domain.
2. It needs a victim attribute: a string, of a shape you may abuse. A resource
   whose every attribute is load-bearing has nowhere to hang the edge.
3. It needs reasoning about phases to be safe. That attribute is ForceNew in
   its provider, so an unknown value there would replace the resource on every
   plan. It does not become unknown, because phases resolve references against
   the ledger before the provider is called. That is a lot of thinking for
   "run this after that".

nivis-demos already has a test helper, `dependsOnId`, that accepts either a
`__ref` or a `__derived` with one matching input. A test had to be taught to
recognise the idiom, which is the symptom.

## The counter-argument, and its limit

Terraform `depends_on` is rightly treated as a smell, and its own documentation
calls it a last resort. That argument holds while data flows. Where no data
flows, inventing a fake data flow is the worst of both: the explicit
declaration is still there, disguised as something else.

## Shape

An edge in the graph that stays out of the provider payload:

```nix
activation = mkResource {
  provider = "nivis-tunnel";
  type = "nixos_activation";
  after = [ instance ];
  config = { stream_id = vars.tunnelStreamId; };   # tells the truth
};
```

`mkResource` already accepts `meta`, which ingest reads only to reject `count`
and `for_each`, so there is room. A dedicated field seems more honest than
hiding an edge in a bag named `meta`.

## Todo

- [ ] Decide the field: `after` on mkResource, or an entry under `meta`
- [ ] Feed it into phase assignment, so an edge with no data flow still moves a
      resource to a later phase
- [ ] Reject a cycle it introduces with the same error a reference cycle gets
- [ ] Test: two resources with no shared data, ordered, in separate phases
- [ ] nivis-demos then drops the derived-that-discards-its-input in 040
