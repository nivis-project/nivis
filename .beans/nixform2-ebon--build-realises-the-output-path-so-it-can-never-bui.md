---
# nixform2-ebon
title: __build realises the output path, so it can never build an unsubstitutable derivation
status: in-progress
type: bug
priority: high
created_at: 2026-09-08T12:00:18Z
updated_at: 2026-09-08T12:39:33Z
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
