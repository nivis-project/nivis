# Spec delta: e2e

## ADDED Requirements

### Requirement: Headline two-provider round trip
The system SHALL pass an end-to-end test in which two providers produce unknown
values on both sides, resolution requires at least 3 apply phases reaching a
fixpoint, and a Nix consumer reads concrete outputs from both providers. The
test SHALL drive the real flake and real fake provider binaries.

#### Scenario: three-phase resolution to fixpoint
- GIVEN the flake topology A -> (Nix) -> B -> (Nix) -> C with a both-providers consumer
- WHEN the phase driver runs to a fixpoint
- THEN exactly 3 phases performed an apply and the loop halted at fixpoint (not a hardcoded count).

#### Scenario: final ledger and consumer are concrete
- WHEN the run completes
- THEN the ledger contains `A.id`, `A.value`, `B.endpoint`, and `C`'s outputs
- AND the `systemConfig` consumer's `recordEndpoint`, `tokenValue`, and `combined`
  are concrete and equal the deterministic provider derivations.

### Requirement: N>2 phases are required, not incidental
The headline test SHALL demonstrate that capping the loop at 2 phases leaves the
later resources and consumer unresolved.

#### Scenario: two-phase cap leaves work pending
- WHEN the driver runs with a 2-phase cap on the headline topology
- THEN `C` and the `combined` consumer value are not resolved and the run does not reach a clean fixpoint.

### Requirement: Cyclic dependency is rejected at fixpoint
The driver SHALL reach fixpoint with cyclic resources unapplied and return an
actionable error naming them, when the dependency graph is cyclic through Nix
(e.g. `A.label` depends on `C`).

#### Scenario: cycle variant names the stuck resources
- GIVEN a plan where `A.label` depends on `C.value` (cyclic with C depending on A)
- WHEN the driver runs
- THEN it returns an error identifying A and C as unresolvable (cycle / missing producer).
