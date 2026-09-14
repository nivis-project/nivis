---
# nixform2-nwnf
title: A replacement destroys a resource before the resources that depend on it
status: todo
type: bug
created_at: 2026-09-14T20:22:58Z
updated_at: 2026-09-14T20:22:58Z
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
