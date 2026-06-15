# Spec delta: executor

## ADDED Requirements

### Requirement: Refresh prior state before planning
Before planning a resource that is present in state, the executor SHALL by
default **refresh** its prior state by reading the resource through the provider
(`ReadResource`) and use the read result as the prior state, so the plan reflects
the real world rather than the stored record. The behavior SHALL be:
- the read returns attributes → those are the prior state (so out-of-band drift
  is planned against, and an unchanged resource is still a no-op);
- the read returns **empty** (the resource was deleted out-of-band) → the
  resource is treated as having no prior state and is planned/applied as a
  **create**;
- a resource not in state → unchanged (a create; not read).
The executor SHALL provide an opt-out (a `--refresh=false` flag, default true) to
plan against stored state without reading the provider. On apply, a refreshed
resource's state SHALL be persisted.

#### Scenario: drift is planned against the real state
- GIVEN a resource in state whose real attributes have drifted out-of-band
- WHEN it is planned with refresh on
- THEN the provider is read and the plan is computed against the read (drifted) state, not the stale stored state.

#### Scenario: an out-of-band deletion re-creates
- GIVEN a resource in state that was deleted outside Nivis (the provider read returns empty)
- WHEN it is planned/applied with refresh on
- THEN it is planned as a create and apply re-creates it.

#### Scenario: refresh can be disabled
- GIVEN a resource in state
- WHEN plan/apply runs with `--refresh=false`
- THEN the provider is NOT read and the stored state is used as the prior state.
