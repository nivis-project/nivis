# Spec delta: executor

## MODIFIED Requirements

### Requirement: Plan engine
The plan engine SHALL, for a ready resource, fetch the provider schema, look up
the resource's **prior state** (its stored attributes, or none if new), encode
config (resolved values known; unresolved refs unknown), call
`PlanResourceChange` with that prior state, and produce a human-readable plan
without side effects. The plan SHALL surface whether the change is a create, an
in-place update, a replacement (the provider's `RequiresReplace`), or a **no-op**
(the planned state equals the prior state with nothing unknown and no replace),
and render them distinctly.

#### Scenario: plan reports computed attrs as unknown
- GIVEN a ready `alpha_token` with `label` known
- WHEN it is planned
- THEN the plan shows `id` and `value` as known-after-apply (unknown now).

#### Scenario: plan distinguishes create, update, replace, and no-op
- GIVEN one resource not in state, one in state with a changed normal attribute, one in state with a changed force-new attribute, and one in state whose config is unchanged
- WHEN they are planned
- THEN the plan marks them as create, update, replace, and no-op respectively.

### Requirement: Apply engine writes computed outputs to state
The apply engine SHALL call `ApplyResourceChange` with the planned state and the
resource's **prior state** (null for a create), extract the now-known computed
outputs, and persist them to the state store. For a replacement it SHALL destroy
the prior resource before creating the new one so none is orphaned. When the plan
is a **no-op** (planned equals prior), the apply engine SHALL NOT call
`ApplyResourceChange` (nor destroy); it SHALL leave the prior state in place and
use the prior attributes as that resource's outputs, so dependents still resolve.
State SHALL be written after each successful apply so a failure mid-run leaves
prior successes recorded.

#### Scenario: apply yields and persists deterministic outputs
- GIVEN a planned `alpha_token` with `label = "rec-X"` (counter 0)
- WHEN it is applied
- THEN state for that resource records `id = "alpha-0"` and `value = "alpha:rec-X:0"`.

#### Scenario: an unchanged resource is a no-op on re-apply
- GIVEN a resource already in state whose config is unchanged
- WHEN it is applied again
- THEN the provider's ApplyResourceChange is not called for it, its state is unchanged, and its prior outputs are still available to dependents.

#### Scenario: partial state persists on mid-run failure
- GIVEN A applies successfully and B then fails
- WHEN the run aborts
- THEN A's computed outputs remain in the state store.
