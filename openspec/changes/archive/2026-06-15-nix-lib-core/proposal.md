# Proposal: nix-lib-core

## Why
Nix is the configuration frontend: users describe resources as Nix values, and
the library serializes them to the JSON IR the executor consumes. This change
builds the round-trip-critical core of the Nix library — resource construction,
the reference system, derived (Nix-computed) values, the `toIR` serializer, and
the flake `plan` interface that accepts the outputs ledger on each phase.

This is what makes the phased-eval loop (E3.5) possible: the phase driver calls
`nix eval .#nixform.plan` with the ledger injected, and on each phase more `__ref`
and `__derived` leaves resolve to concrete values. Without it the loop has no IR
to drive. The module system and full `for_each`/`count` (ROADMAP 1.3/1.4) are
valuable but not on the round-trip path; they can follow as a second change.

## What changes
- `mkResource { provider, type, name, config }`: returns an attrset with stable
  identity (`<provider>.<type>.<name>`) and a thunk exposing the resource's
  computed output attributes as referenceable Nix values, so `r.value` at eval
  time yields a typed `__ref` placeholder the serializer recognizes.
- Reference system: accessing a resource's output attribute produces a `__ref`
  leaf `{ resource, path }`; building a *new* value from one (e.g. string
  interpolation) produces a `__derived` leaf `{ inputs }`. Both survive a phase
  unresolved and become concrete once the outputs ledger supplies their inputs.
- `toIR`: serialize a resource set + provider set + nixConsumers to the canonical
  IR (`docs/IR-CONTRACT.md`), with refs/derived encoded and edges derived from
  `__ref` usage. Output MUST conform to `docs/ir-schema.json`.
- Flake interface: `nixform.plan` is a function of the injected outputs ledger
  (empty on phase 0) producing the IR; refs/derived resolve against the ledger.

## Non-goals
- Module system via `lib.evalModules` (ROADMAP 1.4) — a later change; this change
  takes resources as a plain set.
- `for_each`/`count` expansion (ROADMAP 1.3) — a later change. (The IR already
  forbids these post-expansion; producing them is separate.)
- The executor / phased loop (E3 done / E3.5 next) — this is the producer side.
- Real providers, sensitive-value channel — out of scope here.

## Impact
- New: `nix/lib/` (the library: `mkResource`, refs, `toIR`), `flake.nix`
  exposing `nixform.plan`, and Nix property tests.
- Establishes the eval-time contract: how a Nix author references apply-time
  values and how those become `__ref`/`__derived` in the IR — the input side of
  the round trip E3.5 closes. Verified against the same `docs/ir-schema.json`
  the executor validates, via `tests/ir-conformance/check.py`.
