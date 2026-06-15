# Proposal: nix-lib-modules

## Why
`nix-lib-core` delivered the round-trip-critical half of the Nix library
(mkResource, refs, toIR, plan). This change completes E1 with the two deferred
pieces: meta-arguments with **`for_each`/`count` expansion in Nix**, and a
**module system** so user infrastructure composes across files and merges into
one flat resource graph. These are how real configs are written; they are off
the round-trip critical path (which is why they were deferred) but needed for the
library to be usable beyond a single flat resource list.

## What changes
- Meta-arguments on `mkResource`: `dependsOn`, `lifecycle` (preventDestroy,
  ignoreChanges) carried into the IR `meta`. (Already partly supported; this
  formalizes and tests them.)
- **Expansion in Nix** (IR contract requirement): `mkResources` helpers that
  take `count = N` or `forEach = { <key> = <value>; }` and produce concrete,
  already-expanded instances with deterministic ids `<base>__<key>` (or
  `<base>__<index>` for count). `forEach` maps via `builtins.mapAttrs`. The IR
  never contains `count`/`forEach` — only the expanded instances. A ref into an
  instance is an ordinary `__ref` to the concrete expanded id.
- **Module system**: an `evalModules`-style merge — each module is
  `{ config, tf, lib, ... }: { resources = [...]; providers = {...};
  nixConsumers = [...]; }`. Modules see prior modules' resources via `tf`
  (so a module/consumer can read `tf.<id>` refs), and their outputs merge into a
  single flat graph `toModuleIR` serializes. Conflicting resource ids are an
  error naming the id.

## Non-goals
- Full NixOS option-type system / `mkOption`/`mkMerge` semantics — a minimal
  merge sufficient for composing resource sets, not a general module type system.
- nixpkgs `lib.evalModules` — unavailable (no binary cache, CLAUDE.md §6); a
  self-contained merge in minilib, matching the existing no-nixpkgs decision.
- Changes to the IR shape — expansion and modules produce the same IR; the
  contract already mandates expansion-in-Nix and forbids count/for_each in the IR.

## Impact
- New: `nix/lib/expand.nix` (count/for_each), `nix/lib/modules.nix` (evalModules
  + toModuleIR), minilib additions, property tests for both. Extends the public
  lib surface (`mkResources`, `evalModules`, `toModuleIR`).
- Completes E1. The IR these produce is the same contract artifact the executor
  and phased loop already consume — no downstream changes required.
