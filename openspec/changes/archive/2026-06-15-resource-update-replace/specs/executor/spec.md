# Spec delta: executor

## ADDED Requirements

### Requirement: Create, update, and replace from prior state
The executor SHALL decide a resource's operation from its prior state and the
provider's plan, generically for every resource type (no per-resource code): when
there is **no** prior state for the resource id, it SHALL create; when prior state
exists and the plan does **not** require replacement, it SHALL update the resource
in place; when prior state exists and the plan **requires replacement** (a
force-new attribute changed), it SHALL replace it by destroying the prior resource
and then creating the new one, leaving no orphaned resource. The decision SHALL
derive from `(prior state present?, RequiresReplace?)` returned by the provider,
which itself judges create/update/replace from `(PriorState, ProposedNewState)`
and its schema.

#### Scenario: changed normal attribute updates in place
- GIVEN a resource already in state whose config changes only a non-force-new attribute
- WHEN it is applied again
- THEN the provider receives the stored prior state, the resource is updated in place (not recreated), and state reflects the new attributes.

#### Scenario: changed force-new attribute replaces
- GIVEN a resource already in state whose config changes a force-new attribute
- WHEN it is applied again
- THEN the executor destroys the prior resource and creates the new one, exactly one resource remains, and no prior resource is orphaned.

#### Scenario: a new resource is still created
- GIVEN a resource id with no prior state
- WHEN it is applied
- THEN it is created (prior state null), as before.

## MODIFIED Requirements

### Requirement: Plan engine
The plan engine SHALL, for a ready resource, fetch the provider schema, look up
the resource's **prior state** (its stored attributes, or none if new), encode
config (resolved values known; unresolved refs unknown), call
`PlanResourceChange` with that prior state, and produce a human-readable plan
without side effects. The plan SHALL surface whether the change is a create, an
in-place update, or a replacement (the provider's `RequiresReplace`), and render
them distinctly.

#### Scenario: plan reports computed attrs as unknown
- GIVEN a ready `alpha_token` with `label` known
- WHEN it is planned
- THEN the plan shows `id` and `value` as known-after-apply (unknown now).

#### Scenario: plan distinguishes create from update from replace
- GIVEN one resource not in state, one in state with a changed normal attribute, and one in state with a changed force-new attribute
- WHEN they are planned
- THEN the plan marks them as create, update, and replace respectively.

### Requirement: Apply engine writes computed outputs to state
The apply engine SHALL call `ApplyResourceChange` with the planned state and the
resource's **prior state** (null for a create), extract the now-known computed
outputs, and persist them to the state store. For a replacement it SHALL destroy
the prior resource before creating the new one so none is orphaned. State SHALL be
written after each successful apply so a failure mid-run leaves prior successes
recorded.

#### Scenario: apply yields and persists deterministic outputs
- GIVEN a planned `alpha_token` with `label = "rec-X"` (counter 0)
- WHEN it is applied
- THEN state for that resource records `id = "alpha-0"` and `value = "alpha:rec-X:0"`.

#### Scenario: partial state persists on mid-run failure
- GIVEN A applies successfully and B then fails
- WHEN the run aborts
- THEN A's computed outputs remain in the state store.

### Requirement: Version-neutral provider client interface
The executor SHALL access providers through a version-neutral `provider.Client`
interface exposing `GetSchema`, `Plan`, `Apply`, and `Read`, exchanging
normalized Go types (schema model, attribute maps, diagnostics) rather than
protocol-version-specific protobuf. The plan request SHALL carry an optional
prior state and the plan result SHALL carry whether replacement is required; the
apply request SHALL carry the prior state. Plan/apply/destroy/refresh/codegen
SHALL depend only on this interface.

#### Scenario: v6 backend satisfies the interface
- GIVEN the tfprotov6 backend
- WHEN the executor drives a fake v6 provider through GetSchema/Plan/Apply/Read
- THEN it works through the `provider.Client` interface with no protocol types leaking to callers.

#### Scenario: prior state and replace flow through the interface
- GIVEN a resource with prior state and a force-new change
- WHEN it is planned and applied through `provider.Client`
- THEN the backend sends the prior state to the provider and reports `RequiresReplace`, with no protocol types leaking to callers.
