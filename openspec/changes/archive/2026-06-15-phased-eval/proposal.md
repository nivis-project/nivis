# Proposal: phased-eval

## Why
This is the thesis (DESIGN D3). The single-provider round trip (E3) and the
Nix library that emits ledger-resolving IR (E1) both exist and are individually
proven. This change wires them into the **N-phase fixpoint loop**: re-evaluate
Nix with the accumulated outputs ledger, apply the resources that just became
ready, append their computed outputs, and repeat until no new value resolves.

It generalizes the shallow two-phase case to N phases. The phase count is forced
by Nix-mediated (`__derived`) dependencies — each such hop cannot collapse into
one pass, which is exactly why the loop (not a single apply) is required.

## What changes
- `internal/ledger`: the outputs ledger — load/save the contract's
  `{ phase, outputs }` JSON (0600 file), append a resource's computed outputs,
  and answer "is `<id>.<attr>` known?". Sensitive outputs are kept out of any
  world-readable surface (the ledger file is 0600; sensitive values referenced,
  not embedded in IR).
- `internal/phase`: the phase driver. A `NixEvaluator` interface produces the
  IR for the current ledger (real impl shells `nix eval .#nixform.plan`; a stub
  is used for hermetic unit tests). The loop: eval → ingest → find ready
  resources (TF→TF resolved, no derived leaves) → plan+apply via the executor →
  append outputs → repeat while a phase resolved ≥1 new output and work remains.
- Fixpoint & error detection: halt when a phase yields no new resolved value; if
  unresolved refs/resources remain at halt, return an actionable error naming
  the stuck resources/attrs (unresolvable value or dependency cycle).
- `*→Nix` feedback verification: confirm a re-evaluated Nix expression (a
  `__derived` value, e.g. a string built from a computed output) becomes a
  concrete value in a later phase's IR and is consumable downstream.
- A real-Nix integration test that runs the flake + fake providers through the
  loop, asserting the phase count and the resolved consumer values.

## Non-goals
- The full headline two-provider e2e with all its assertions (E4b) — this change
  proves the loop mechanism and a multi-phase run; E4b is the formal milestone
  exit test with the cycle variant, destroy/refresh, etc.
- Refresh/destroy/CLI (E3b), real providers (network-gated), schema codegen (E2).
- Sensitive-value private re-eval channel beyond keeping the ledger 0600 and out
  of the store — the deep `__sensitiveRef` argstr path is exercised only where a
  sensitive output actually feeds a re-eval (not in the fake topology here).

## Impact
- New: `internal/ledger`, `internal/phase`, and a `cmd/nixform` entry exercised
  by the integration test (or a test-only driver). New behavior: the executor
  now resolves a whole graph across phases, not one resource.
- Completes the conceptual core of the PoC. After this, E4b formalizes the exit
  criterion and E3b/E2/E4c are breadth/polish off the critical path.
