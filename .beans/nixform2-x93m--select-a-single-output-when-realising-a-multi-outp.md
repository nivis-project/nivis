---
# nixform2-x93m
title: Select a single output when realising a multi-output derivation
status: todo
type: task
priority: low
tags:
    - discovered
created_at: 2026-09-08T12:53:46Z
updated_at: 2026-09-08T12:53:46Z
parent: nixform2-kovh
---

Deferred from `nixform2-ebon` / OpenSpec change `realise-build-leaves-from-drv`, design Decision 5.

`nix-store --realise <drv>` builds **every** output of the derivation. For a single-output image that is exactly right; for `drvFile d "path"` on a multi-output derivation it can build a large output nobody asked for (a `doc` or `dev` output, say).

## Why it was not fixed in that change

No current configuration hits it: `drv`/`drvFile` are used on single-output derivations. Fixing it speculatively would have added CLI-version dependence for no present benefit.

## What it needs

- Selecting one output requires the newer CLI form (`nix build "$drv^out"`) rather than `nix-store --realise`, which changes the minimum Nix version nivis assumes.
- The leaf does not record WHICH output its path belongs to; `drv d` interpolates the default output, but `drv d.dev` would record a path from another output with the same `drvPath`. So the leaf likely needs the output NAME too, which is another IR contract change.

## Acceptance criteria

- A `__build` leaf over one output of a multi-output derivation builds only that output.
- The minimum Nix version nivis requires is stated wherever it is documented, if this raises it.
- A hermetic test with a two-output derivation asserts the unused output is not built.
