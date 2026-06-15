# Proposal: define-ir-contract

## Why
The canonical JSON IR is the single contract between the Nix library (Epic 1),
the schema codegen (Epic 2), and the Go executor (Epic 3/3.5). Three workstreams
depend on its exact shape; once it is stable they can proceed in parallel. An
underspecified IR is the most likely way this project fragments, so it is
authored and frozen before producers/consumers are built.

This change establishes `docs/IR-CONTRACT.md` as the source of truth and adds
the conformance requirements both sides must meet.

## What changes
- Establish the IR top-level shape, the typed reference encoding (`__ref`), the
  derived-value encoding (`__derived`), the unknown-value mapping toward
  providers, `for_each`/`count` expansion timing (in Nix), sensitive-value
  handling across the JSON/store boundary, and the outputs-ledger injection
  format.
- Define conformance obligations: a Nix property test for `toIR` output and a Go
  `IngestIR` validator.

## Non-goals
- Implementing `toIR` (Epic 1.5) or `IngestIR` (Epic 3a.1) — this change defines
  the contract and its conformance tests' *requirements*, not the producers.
- Remote state, registry download, real providers — out of PoC scope.
- Any provider-specific schema detail — codegen (Epic 2) consumes this contract;
  it does not alter it.

## Impact
- New: `docs/IR-CONTRACT.md` (the contract), `schemaVersion: 1`.
- Downstream changes in E1, E2, E3, E3.5 must conform; breaking the shape later
  requires a new change here first and a `schemaVersion` bump.
