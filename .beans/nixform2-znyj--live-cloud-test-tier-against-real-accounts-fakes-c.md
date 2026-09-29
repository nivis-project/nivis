---
# nixform2-znyj
title: Live cloud test tier against real accounts (fakes cannot prove AWS agrees)
status: todo
type: feature
created_at: 2026-09-24T15:59:10Z
updated_at: 2026-09-24T15:59:10Z
---

The hermetic suite proves MECHANISM. It cannot prove that the cloud agrees,
because the fakes encode our own understanding of S3, STS and IAM. A misreading
produces a fake that agrees with the misreading, a green suite, and a broken
production path.

## Two concrete cases, both from 2026-09-24

1. `nixform2-rlbz` (SSE-KMS). The deny policy was derived by reading
   `terraform-aws-s3` (pinned `945d79d`) and `internal/fakes3.DenyMismatchedSSE`
   was built to match it. If the `Null` condition had been misread, the fake would
   have agreed and the e2e would have passed while every real write 403'd. What
   actually confirmed it was a manual run against the real bucket.
2. `nixform2-nd2c` (assume-role). The hermetic e2e gates a fake bucket on the
   SigV4 access key the fake STS issues, which proves the mechanism. Confirming it
   against a real member-account bucket in the landing zone stays manual, and is
   recorded as such in that bean.

There is also existing practice to formalize: `nixform2-807d` records "VERIFIED
LIVE against AWS (account 076504012268)" by hand. The verification already
happens; it is just not repeatable or automatic.

## Shape: split the test code from the environment

A private repo full of duplicated Go tests would drift against this repo. The part
that genuinely cannot be public is the ENVIRONMENT DESCRIPTION.

```
nivis (public)                          private repo
|- hermetic tests         (today)       |- the accounts' fixtures as config
`- live tests, build-tagged (new)       |- env files naming them
   //go:build live                      `- the runner, schedule, secrets
```

One copy of the test code, next to the code it tests. The private repo describes
and provisions the world it runs against.

## Tier by cost; the tiers differ by orders of magnitude

| tier | creates | cost | when |
|------|---------|------|------|
| state backend: real S3/KMS/STS, FAKE providers | two S3 objects | cents | now |
| real providers: AWS/Hetzner resources | real infrastructure | money, slow, teardown risk | with nixform2-2vc3 |

The first tier is the one that keeps coming up and it is nearly free.
`nix/example/remote-state.nix` is already exactly this shape: real backend, fake
providers, no infrastructure created. Everything in `rlbz` and `nd2c` is testable
there for pennies with no blast radius.

## Two constraints that decide whether this is worth it

- **This environment cannot run it.** CLAUDE.md section 6: egress is allowlisted
  and the OpenTofu registry and Nix cache are unreachable. Live tests can never be
  part of the normal loop; they run elsewhere, on a schedule, with feedback
  arriving hours later rather than at commit time.
- **A scheduled suite nobody reads is worse than none**, because it manufactures
  confidence. It needs a path that puts a failure in front of a person. Without
  that, this is the manual practice we already have plus maintenance.

## Cheaper thing worth doing first, possibly instead

Record real AWS responses once and replay them in the hermetic suite (a golden
corpus). That kills the specific "our fake believes something false" failure with
no recurring spend and no second CI. It cannot catch IAM or policy interactions or
behaviour that drifts over time, so it complements a live tier rather than
replacing it, but it is a fraction of the work and targets the exact failure mode
that worries me most.

## Not in scope here

The expensive tier belongs with `nixform2-2vc3` (M2 real provider support). This
bean is about establishing the cheap tier and the split above.
