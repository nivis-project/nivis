# Proposal: headline-e2e

## Why
This change formalizes the milestone exit criterion (ROADMAP / docs/TESTING.md)
into a single named e2e test: two providers, unknown values originating on
**both** sides, resolution across **≥3 phases** to a fixpoint, and a Nix-side
consumer reading outputs from **both** providers. The phased-eval loop (E3.5)
already proved the mechanism; this makes it the official, assertion-complete exit
test and adds the **cycle-rejection** variant.

## What changes
- A dedicated e2e at `tests/e2e/` driving the real flake `nixform.plan` and the
  real fake provider binaries through the phase driver, asserting the full set
  from docs/TESTING.md (minus destroy/refresh — see Non-goals):
  - exactly 3 apply phases, halting at fixpoint (not a hardcoded count);
  - a 2-phase cap leaves later resources pending (proves N>2 is required);
  - the final ledger contains `A.id`, `A.value`, `B.endpoint`, `C.*`;
  - `systemConfig` resolves to concrete `recordEndpoint`/`tokenValue`/`combined`
    from both providers, matching the deterministic derivations.
- A **cycle variant**: a second flake plan attr (`nixform.planCycle`) where
  `A.label` depends on `C` (making the graph cyclic through Nix). The driver
  reaches fixpoint with A and C unapplied and returns an actionable error
  naming A and C.

## Non-goals
- `destroy` (reverse-order) and `refresh` (`ReadResource`) — these need executor
  engines that do not exist yet; the roadmap parks them in E3b. Tracked as beans
  nixform2-xtgz; the headline test is complete except for those two assertions,
  which E3b will add. This is called out explicitly so the deferral is visible,
  not silent.
- Real providers / registry (network-gated), schema codegen (E2).

## Impact
- New: `tests/e2e/` (the e2e test + any topology fixtures) and a
  `nixform.planCycle` attr in the flake for the cycle variant.
- Passing this test == the milestone's core round-trip thesis is validated and
  trustworthy. Remaining milestone work (destroy/refresh, codegen, docs) is
  breadth/polish off the critical path.
