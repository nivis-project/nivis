---
# nixform2-xqy1
title: Let a domain assert on what came back
status: todo
type: task
created_at: 2026-09-14T20:43:05Z
updated_at: 2026-09-14T20:43:05Z
---

Nivis applies a resource, stores what the provider read back, and never asks
whether it is what was meant. The domain author has no way to say so either.

## What prompted it

An image was built, uploaded, imported and registered, and the machine booted
nothing. The upload had been truncated. Every step reported success, because
every step had been told what to do and nothing had been told what to expect.

The check that would have caught it is one comparison: the number of bytes the
import read against the number of bytes we built. A human ran it afterwards.

## The shape

A domain declares an expectation over a resource computed attribute, checked
after apply:

```nix
expect = [
  { got = snapshot.refAttr "volume_size"; ge = 5; }
];
```

This is deliberately not a policy engine. It is the assertion an author would
otherwise write in a comment and never run.

## Where it stops

For the case that prompted it, there is nothing to assert against:
`aws_ebs_snapshot_import` exposes `volume_size` and not `disk_image_size`. The
byte count lives only in `DescribeImportSnapshotTasks`, outside the resource.
So this feature would not have caught THIS failure, and saying so is the point:

- assertions catch what a provider surfaces
- nixform2-sqco gives nivis the expectation to compare against
- neither replaces making the failure impossible at the source, which for this
  one is a checksum on the upload so S3 refuses an incomplete object
  (nivis-demos-u4x9)

Three different layers, and only the third actually closes this hole. The other
two close the ones where the provider does surface a number.

## Todo

- [ ] Decide the form: an `expect` list on a resource, or assertions at domain
      level over any resource output
- [ ] Evaluate after apply, against the ledger, so a failed assertion names the
      resource and both values
- [ ] Decide what a failed assertion does: fail the apply, or report and leave
      the state as applied
- [ ] Test: an assertion that holds, one that fails, and one whose attribute
      the provider never populated
