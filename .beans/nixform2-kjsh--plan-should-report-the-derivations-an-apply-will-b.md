---
# nixform2-kjsh
title: plan should report the derivations an apply will build
status: todo
type: task
priority: normal
tags:
    - discovered
created_at: 2026-09-08T12:53:46Z
updated_at: 2026-09-08T12:53:46Z
parent: nixform2-kovh
---

Deferred from `nixform2-ebon` / OpenSpec change `realise-build-leaves-from-drv`, design Decision 5.

`nivis plan` does not say that the coming apply will build anything. Realising lives in `applyOne`, so `plan` stays side-effect free — correct, but it means the surprise moves from "a confusing failure" (before the fix) to "an unexplained wait" (after it): a user plans, sees three resources, runs apply, and waits twenty minutes for a 2 GB image with no warning that a build was coming.

## What it needs

- `plan` already evaluates the config, so the `__build` leaves reachable this phase are known without building anything. A line such as `will build: 1 derivation (nixos-image-…)` is cheap.
- Deciding whether to report only leaves whose output path is currently invalid (accurate, requires a store check per leaf) or every leaf (cheaper, noisier).
- Whether `plan` should report the estimated download/build size. Nix can report this (`nix path-info --derivation --closure-size`, or a dry-run build) — worth checking whether it is worth the extra call.

## Acceptance criteria

- `plan` reports which derivations an apply would build, or reports nothing when there are none.
- `plan` still performs no build and no state mutation.
- Covered by an e2e over `nivis.buildProbe`, which has a never-built leaf by construction.
