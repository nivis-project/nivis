# Proposal: ir-backend-block

## Why
M2 (team-ready) needs a remote state backend (B1, S3 first). The bean and the
roadmap require it be configured **in the flake, not via env soup**, so where
state lives must be declared in the Nix config and reach the executor. The IR is
the frozen seam between Nix and Go, so the backend declaration belongs in the IR
as a new top-level field. This change adds that field to the contract, the Nix
library, and Go ingest/validation. It deliberately ships **before** the S3 backend
implementation (a separate change), because the IR is a contract and the spec must
change before the shape.

## What changes
- A new **OPTIONAL** top-level IR field `backend`: a static object declaring where
  state is stored, e.g.:
  ```jsonc
  "backend": { "type": "s3", "bucket": "my-state", "key": "prod/app", "region": "eu-west-1" }
  ```
  - `backend` is **static configuration**, known before any evaluation: its leaves
    are plain JSON scalars/objects, never `__ref`/`__derived`/unknown (the
    executor must know where state lives before it evaluates anything). The IR
    validation SHALL reject a `backend` containing a ref/unknown leaf.
  - `type` is required and identifies the backend; other keys are
    backend-specific and **not** interpreted by the IR layer (the S3 backend
    change defines and validates the s3 keys). Credentials are NEVER in `backend`
    (they come from the provider/AWS credential chain); only the location is.
  - Absent `backend` means the local file store (today's default), so existing
    configs and the whole test suite are unaffected.
- This is an **additive, non-breaking** change: `schemaVersion` stays `1`. An
  optional field an older executor ignores does not break the contract (the
  contract bumps the version only on a breaking change). `docs/IR-CONTRACT.md`
  documents it as optional.
- `nix/lib/toIR.nix` (and `toModuleIR`) accept an optional `backend` argument and
  emit it only when set. `internal/ir` parses it into the `Document`/`Graph` and
  validates the static-only rule.

## Non-goals
- Implementing any backend (S3 or otherwise) or reading state through it. That is
  the follow-up change `s3-state-backend`. This change only carries the
  declaration through the contract and validates its shape.
- Interpreting backend-specific keys (bucket/key/region) here; the IR layer only
  enforces `type` present and no refs/unknowns.

## Impact
- `docs/IR-CONTRACT.md` + `docs/ir-schema.json`: the optional `backend` field and
  its static-only rule.
- `nix/lib/toIR.nix`, `nix/lib/modules.nix` (toModuleIR): optional `backend` pass-through.
- `internal/ir/types.go`, `internal/ir/ingest.go`: parse + validate `backend`.
- Tests: Nix property test (a config with a backend serializes it; without, it is
  absent), Go ingest tests (valid backend parses; a backend with a `__ref` leaf or
  missing `type` is rejected with a clear error), and the conformance check.

Changelog: Added an optional `backend` field to the IR (and `toIR`) declaring where
state is stored (e.g. an s3 bucket/key/region), so a remote state backend is
configured in the Nix flake rather than via flags or env vars. Static only (no
refs); absent means the local file store.

Docs impact: modifications only - `docs/IR-CONTRACT.md` documents the new optional
field (it is part of the frozen contract). No new user-facing document yet; the S3
backend change (with the user-facing `nivis.backend` flake usage) is where a
"remote state" doc/section is decided.
