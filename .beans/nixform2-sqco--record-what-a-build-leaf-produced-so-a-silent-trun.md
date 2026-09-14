---
# nixform2-sqco
title: Record what a __build leaf produced, so a silent truncation cannot pass
status: todo
type: task
created_at: 2026-09-14T20:23:19Z
updated_at: 2026-09-14T20:23:19Z
---

A `__build` leaf is the one place where nivis knows something the provider never
will: the exact file it handed over. It currently keeps none of it.

## What happened

In nivis-demos `040_tunnel_target`, an image was built and uploaded to S3 by
`aws_s3_object`. The apply reported success. The upload had stopped at
1,053,818,880 bytes of a 1,890,006,016 byte VHD.

Nothing caught it. Not the provider, which had no expectation to compare
against, and not nivis, which had built the file and then forgotten its size.
A truncated VHD imports without complaint, because the header at the front
still declares a 4 GiB disk, so what came out was an AMI that booted nothing
and a machine that wrote not one line to its serial console.

The cause was only found by arithmetic on the recorded etag: both uploads used
5 MiB parts, the good one counted out at 506 parts for its size, the bad one
carried 201 where 361 were due.

## What to record

For every `__build` leaf, in the ledger next to the resource that consumed it:

- the store path
- the size in bytes
- a content hash

## What that buys

- a later apply can see the local artefact differs from what was deployed,
  without depending on a resource attribute like a key name to change
- an operator has a number to compare against, instead of reverse engineering
  an etag
- it is the generic form of the fix nivis-demos had to make by hand, which was
  to put the image store hash into the S3 key so the artefact became part of
  the downstream resource identity

## Not in scope

Verifying the remote object. That is provider specific, and the provider can
already be told to do it: `aws_s3_object` accepts `checksum_algorithm`, which
makes S3 validate each part server side.

Comparing the recorded number against what a provider read back is nixform2-xqy1.
This bean produces the expectation; that one spends it. Note that for the
failure which prompted both, neither suffices: `aws_ebs_snapshot_import` does
not expose the byte count it processed, so the only thing that closes it is a
checksum at upload time.

## Todo

- [ ] Record store path, size and hash for each `__build` leaf in the ledger
- [ ] Surface the size next to the node during apply, so a step that moves
      2.6 GB says so rather than appearing to hang
- [ ] Test: a build output that changes is visible in the ledger diff
