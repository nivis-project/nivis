---
# nixform2-ebon
title: __build realises the output path, so it can never build an unsubstitutable derivation
status: completed
type: bug
priority: high
created_at: 2026-09-08T12:00:18Z
updated_at: 2026-09-08T13:04:09Z
parent: nixform2-kovh
---

A `__build` leaf carries only the derivation's **output path**, and the executor realises it with:

```go
// internal/phase/evaluator.go:84
cmd := exec.CommandContext(ctx, "nix-store", "--realise", storePath)
```

`nix-store --realise <outputPath>` can use a path that already exists, or substitute it from a binary cache. It **cannot build it**: an output path does not tell Nix how to produce it. Building requires the `.drv`.

So a `__build` leaf pointing at anything not already built and not in a cache fails:

```
error: phase 1: apply "aws.aws_s3_object.image": realise build ...
error: path '/nix/store/s999...-nixos-image-amazon-...' is required, but there is no substituter that can build it
```

This contradicts what the mechanism promises. From `nix/lib/ref.nix`:

> buildLeaf :: store-path-string -> __build leaf. The executor realises this path (builds the derivation) before apply

It does not build the derivation. It only substitutes.

## Why this was never hit before

`infra`'s 020_core_host masks it by accident. The hcloudimage provider requires `image_sha256` next to `image_path`, so the domain does:

```nix
image_sha256 = builtins.hashFile "sha256" "${coreHostImage}/${imageFile}";
```

`hashFile` must read the file, so Nix **builds the image during evaluation** — before the executor runs. By the time `nix-store --realise` is called the path is already valid and the call is a no-op. `infra` documents that eval-time build as a wart ("the one place we break nivis's deferred-build design"); it is also the only reason the realise appears to work.

The AWS provider needs no hash, so `nivis-demos`' EC2 demo has no `hashFile` and nothing forces the build. It is the first configuration to actually exercise deferred build, and it fails immediately.

## Proposed fix

- `nix/lib/ref.nix`: have `buildLeaf` / `drv` / `drvFile` carry the derivation's `drvPath` alongside the output path.
- `internal/phase/evaluator.go`: realise the `drvPath` when present (`nix-store --realise <drv>` builds and prints the output path); fall back to the current behaviour when it is absent, so existing IRs keep working.

## Acceptance criteria

- A configuration whose `__build` leaf points at a locally-built, never-cached derivation applies in ONE run, with no pre-build step.
- The IR contract documents the added field.
- A test covers the unsubstitutable case — the current tests pass only because their paths are already in the store.
- `infra` can then drop `hashFile` if the provider ever stops requiring the hash; not required by this change.

## Context

Found while applying the Vaultwarden EC2 demo in `nivis-demos`, whose whole claim is "the machine is a derivation, in one apply". Pre-building the image works around it but disproves the claim, so the demo is blocked on this.

## Summary of Changes

Shipped as OpenSpec change `realise-build-leaves-from-drv` (archived:
`openspec/changes/archive/2026-09-08-realise-build-leaves-from-drv/` in the
`nivis` store). Commit `6e16a97e`.

**The mechanism, proven from both sides** before writing any code, against a
derivation whose name had never existed anywhere:

```
nix-store --realise <OUTPUT PATH>   don't know how to build these paths / no substituter   ✗
nix-store --realise <DRV PATH>      building '…drv'… → output valid, content correct       ✓
```

**Why the fix is forced, not chosen.** I checked whether the executor could
recover the derivation itself and avoid touching the frozen IR:

```
after a plain `nix eval`:  the .drv IS in the store   (evaluation instantiates it)
nix-store --query --deriver <invalid outPath>
  ▶ error: path '…' is not valid                      ← deriver mapping exists only for VALID paths
```

The store holds the recipe but cannot be asked which recipe makes an unbuilt
path, and the hashes are unrelated by construction. The leaf must carry it — and
doing so is free, since obtaining `"${d}"` already instantiates the same
derivation during evaluation.

**What was built**

- `nix/lib/ref.nix`: `buildLeaf` takes `{ path, drv }`; `drv`/`drvFile` pass
  `d.drvPath`. The comment claiming the leaf's "path exists at evaluation" is
  corrected — evaluation fixes the path STRING; the artifact may not exist. That
  false premise is why an output-path-only leaf looked sufficient.
- `internal/phase`: the `Realiser` seam takes a `Build{Path, Drv}`; the nix
  realiser realises the DERIVATION when present and the output root otherwise;
  `realiseTarget` makes that choice a pure, unit-tested function.
- Already-present paths are skipped (no subprocess on a built stack); the path is
  verified after building; `Building …` / `Built …` is reported (a realise is
  silent, and silence during a 2 GB image build is indistinguishable from a hang);
  a legacy leaf that cannot be substituted explains that the Nix library predates
  the field, because consumers pin that library to a tag while installing the CLI
  separately.
- `docs/ir-schema.json`: **the schema never modelled this leaf at all.**
  `configTree` dispatched on `__ref`/`__derived`/`__sensitiveRef`, so a `__build`
  leaf validated only accidentally, as a generic object, and a malformed one
  produced "no branch matched". It now has a `build` `$def` and dispatch branch
  like the others, with `path` required and `drv` optional for compatibility —
  plus valid fixtures for both shapes and an invalid one proving a malformed leaf
  reports precisely.

**Acceptance criteria**

- One-run apply of a never-cached derivation: done —
  `tests/e2e/build_realise_test.go` `TestApplyBuildsANeverBuiltDerivation`.
- The IR contract documents the field: done, plus the corrected premise.
- A test covers the unsubstitutable case: done, and it **cannot decay**. The probe
  is named uniquely per run (a bare `builtins.derivation`, no nixpkgs, builds in
  milliseconds), the test asserts the path is invalid *before* it runs anything,
  and it reads the leaf out of the evaluated config rather than duplicating the
  derivation. I also verified it FAILS without the fix, reproducing this bean's
  error — the previous proof used a stub realiser, and the original change's
  `tasks.md` said so.
- `infra` can drop `hashFile` when its provider stops requiring the hash: now
  unblocked, not required.

**Concrete confirmation on the documented path**: `nivis.ec2`'s
`aws_s3_object.source` leaf now carries the NixOS image's derivation path, and
that image's output path is **not valid** on this machine — so before this change
an apply here would have failed exactly as reported, and now it builds.

**Deferred, with reasoning** (design Decision 5), each filed: `nixform2-854m`
(the state lock is held across the build), `nixform2-h54w` (GC-root the realised
output), `nixform2-x93m` (select one output of a multi-output derivation),
`nixform2-kjsh` (`plan` should say what an apply will build).
