---
# nixform2-znyj
title: Live cloud test tier against real accounts (fakes cannot prove AWS agrees)
status: todo
type: feature
priority: normal
created_at: 2026-09-24T15:59:10Z
updated_at: 2026-10-15T19:10:44Z
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


## Re-scoped 2026-10-15: cheap tier only

The expensive tier of this bean (real providers creating real infrastructure) is
**superseded by nixform2-q29p**, which drives nivis against real third-party
providers whose backends run inside a nixosTest: hermetic, in CI, feedback at
commit time, no credentials, no spend, no teardown risk. It beats a scheduled
live suite on every axis for the failure mode this bean was written about, and
it answers this bean's own worry directly: "a scheduled suite nobody reads is
worse than none, because it manufactures confidence."

That tier was deferred here to nixform2-2vc3, which is now completed, so it was
unowned and drifting.

**What stays, and why it is not covered by q29p.** The cheap tier is real
S3/KMS/STS with FAKE providers, testing nivis's own STATE BACKEND rather than
any provider. The two cases this bean was written from are both backend work:

- nixform2-rlbz: SSE-KMS deny policy, derived by reading terraform-aws-s3
  source. If the Null condition had been misread, internal/fakes3 would have
  agreed and every real write would have 403'd.
- nixform2-nd2c: assume-role, with internal/fakests issuing the SigV4 key.

Nothing in libvirt or postgres touches SigV4, KMS key policies or STS. MinIO
would narrow the gap (real S3 API, real SigV4) without closing it, because its
KMS is not AWS KMS semantics and its STS is not AWS STS, which are exactly these
two cases.

The golden-corpus idea stays here too: it targets AWS fidelity, which is this
bean's axis.

## Correction: the egress constraint cited above is false

"This environment cannot run it. CLAUDE.md section 6: egress is allowlisted and
the OpenTofu registry and Nix cache are unreachable."

Both are reachable and were measured on 2026-10-09: cache.nixos.org,
registry.opentofu.org, registry.terraform.io and proxy.golang.org all answer,
and a provider artifact downloads end to end from its GitHub release. CLAUDE.md
section 6 was corrected in commit caf8d75 (bean nixform2-irf4). This bean was
one of the four nixform2-vy9f names as resting on the false claim.

So the cost reasoning above needs re-reading: "live tests can never be part of
the normal loop" was argued partly from a constraint that does not exist. What
remains true is the money, the teardown risk and the credentials.
