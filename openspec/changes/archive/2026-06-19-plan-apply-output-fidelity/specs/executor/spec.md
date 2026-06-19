# Spec delta: executor

## MODIFIED Requirements

### Requirement: Apply result groups applied nodes by phase
The phase driver's apply result SHALL report, in addition to the flat ordered list
of applied ids and the phase count, the ids applied in **each** phase, in phase
order, and for each applied id whether it was a **resource** (planned/applied) or
a **datasource** (read), and for each applied **resource** the **operation** it
resolved as (create, in-place update, replace, or no-op). This is reporting
metadata only: it SHALL NOT change which nodes are applied, the order, or any
apply behaviour. It exists so a caller can render the fixpoint (which nodes
resolved in which phase), distinguish a read from a create, and report the true
change type rather than assuming every applied node is a create.

#### Scenario: per-phase grouping reflects the fixpoint
- GIVEN a run that applies resources across three phases (a Nix-mediated chain)
- WHEN it completes
- THEN the result reports three phase groups, each listing the ids applied in that phase, and their concatenation in phase order equals the flat applied list.

#### Scenario: a datasource read is marked as a read
- GIVEN a run that reads a datasource and applies a resource
- WHEN it completes
- THEN the result marks the datasource id as a read and the resource id as an applied resource.

#### Scenario: an applied resource carries its operation
- GIVEN a first run that creates a resource and a second run of the same unchanged config
- WHEN each completes
- THEN the first run reports the resource's operation as create, and the second run reports it as no-op (not create), with its stored id and computed values unchanged between the two runs.

### Requirement: Plan engine
The plan engine SHALL, for a ready resource, fetch the provider schema, look up
the resource's **prior state** (its stored attributes, or none if new), encode
config (resolved values known; unresolved refs unknown), call
`PlanResourceChange` with that prior state, and produce a human-readable plan
without side effects. The plan SHALL surface whether the change is a create, an
in-place update, a replacement (the provider's `RequiresReplace`), or a **no-op**
(the planned state equals the prior state with nothing unknown and no replace),
and render them distinctly.

When planning an already-applied stack, the plan SHALL read each side-effect-free
**datasource** whose inputs are known and seed its outputs into the resolution
ledger before classifying resources, using the same readiness determination the
apply loop uses. A resource whose config is fully resolvable only because it reads
a datasource SHALL therefore be planned against its provider and reported with its
true operation (a no-op when its config is unchanged), NOT reported as an update
merely because a datasource it depends on was not read. Datasource reads during
plan SHALL remain side-effect free: datasources are never planned, applied, or
written to state.

#### Scenario: plan reports computed attrs as unknown
- GIVEN a ready `alpha_token` with `label` known
- WHEN it is planned
- THEN the plan shows `id` and `value` as known-after-apply (unknown now).

#### Scenario: plan distinguishes create, update, replace, and no-op
- GIVEN one resource not in state, one in state with a changed normal attribute, one in state with a changed force-new attribute, and one in state whose config is unchanged
- WHEN they are planned
- THEN the plan marks them as create, update, replace, and no-op respectively.

#### Scenario: a datasource-dependent resource plans as no-op when unchanged
- GIVEN an applied stack with a datasource feeding a resource's config, re-planned with the config unchanged
- WHEN plan runs
- THEN the datasource is read into the ledger, the dependent resource is planned against its provider and reported as a no-op, not as an update.
