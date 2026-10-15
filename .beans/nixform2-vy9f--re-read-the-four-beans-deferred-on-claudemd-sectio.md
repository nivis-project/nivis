---
# nixform2-vy9f
title: Re-read the four beans deferred on CLAUDE.md section 6's false network claim
status: todo
type: task
priority: normal
tags:
    - discovered
created_at: 2026-10-08T21:33:49Z
updated_at: 2026-10-15T19:10:44Z
---

Discovered while shipping `correct-env-constraints` (bean nixform2-irf4).

Section 6 claimed the OpenTofu registry and the Nix binary cache were
unreachable. Both are reachable, and a real provider artifact downloads end to
end. The claim is now corrected, but it had already propagated: four beans cite
section 6 as the reason their work was deferred or scoped out. Each needs
re-reading now that the premise is gone.

- `nixform2-m83a` (C3 provider registry integration): "NETWORK-GATED per
  CLAUDE.md S6 (the registry is not reachable in the dev sandbox; use fakes
  there)."
- `nixform2-8umq` (E2.4 registry resolve/download/verify/cache): "the OpenTofu
  provider registry is NOT reachable in this environment, so real-provider
  download is out of PoC scope." Note this one looks already superseded:
  `openspec/specs/registry/spec.md` specifies the capability and it is marked
  completed. Confirm, and close the loop if the body is merely stale.
- `nixform2-28sn` (package as flake apps): flake-app packaging "deferred"
  because the binary cache was believed unreachable. `nix build` of four flake
  outputs works today, so the stated blocker is gone.
- `nixform2-znyj` (live cloud test tier): repeats "the OpenTofu registry and Nix
  cache are unreachable" as part of its reasoning.

## What to do

For each: read it against what is now measured, then either reword the body so
it no longer rests on the false claim, or reopen/rescope the work if the only
reason it was parked has evaporated. This is judgement per bean, not a sed.

## Why this is not just tidying

A wrong environment claim in the instruction file produced confident wrong
plans. Correcting the file does not retract the plans already made on it. The
cost of nixform2-irf4 is mostly here, in the deferrals, not in the sentence.


## Progress

- nixform2-znyj: done 2026-10-15. Re-scoped to its cheap tier, expensive tier
  superseded by nixform2-q29p, and the false egress claim struck with the
  measured replacement recorded.

Still to re-read: nixform2-m83a, nixform2-8umq, nixform2-28sn.
