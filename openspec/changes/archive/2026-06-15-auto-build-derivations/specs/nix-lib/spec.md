# Spec delta: nix-lib

## ADDED Requirements

### Requirement: nivis.drv marks a build-output value
The library SHALL provide `drv` such that `drv <derivation>` (optionally
`drv <derivation> { file = "<relative-path>"; }`) returns a typed **`__build`
leaf** carrying the absolute store path of the derivation's output (the inner
file when `file`/`passthru.filePath` applies). The leaf serializes to
`{ "__build": { "path": "<store-path>" } }`. Unlike `__ref`/`__derived`, a
`__build` leaf is a **known** value (its path exists at evaluation) — it SHALL
pass through `resolve` unchanged and is realised by the executor before apply, not
resolved against the outputs ledger. It exists so an author marks "this value is a
build output that must be realised" explicitly, rather than relying on string
heuristics.

#### Scenario: drv yields a __build leaf
- WHEN `drv image` is evaluated for a derivation `image`
- THEN it returns `{ __build = { path = "<image store path>"; }; }`, and `toIR`
  emits that leaf verbatim.

#### Scenario: a __build leaf survives a phase unresolved-but-known
- GIVEN a resource config containing a `__build` leaf and a ledger missing other inputs
- WHEN `toIR` resolves the config against that ledger
- THEN the `__build` leaf is unchanged (it is not ledger-dependent), ready for the executor to realise.
