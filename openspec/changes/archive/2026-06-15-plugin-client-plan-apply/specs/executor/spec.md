# Spec delta: executor

## ADDED Requirements

### Requirement: Provider plugin client over go-plugin v6
The executor SHALL spawn a provider binary and complete the go-plugin/gRPC
protocol-6 handshake (matching the tfprotov6 server's magic cookie and protocol
version), obtaining a working `tfplugin6.ProviderClient`. Clients SHALL be pooled
by provider identity and closed on shutdown.

#### Scenario: spawn and handshake with a fake provider
- GIVEN the built `provider-alpha` binary
- WHEN the manager starts it and performs the handshake
- THEN `GetProviderSchema` over the returned client succeeds and lists `alpha_token`.

#### Scenario: pooled by identity
- WHEN two resources of the same provider are processed
- THEN the manager reuses one spawned process, not two.

### Requirement: Unknown values presented to the provider
The executor SHALL present any unresolved `__ref` attribute to the provider as
the tfprotov6 unknown value, never as the `__ref` JSON, when planning a resource
whose config still contains that unresolved reference.

#### Scenario: unresolved ref becomes unknown at plan
- GIVEN a resource whose `from` is an unresolved `__ref`
- WHEN the plan engine encodes config for `PlanResourceChange`
- THEN `from` is sent as the protocol unknown value.

### Requirement: Plan engine
The plan engine SHALL, for a ready resource, fetch the provider schema, encode
config (resolved values known; unresolved refs unknown), call
`PlanResourceChange`, and produce a human-readable plan without side effects.

#### Scenario: plan reports computed attrs as unknown
- GIVEN a ready `alpha_token` with `label` known
- WHEN it is planned
- THEN the plan shows `id` and `value` as known-after-apply (unknown now).

### Requirement: Apply engine writes computed outputs to state
The apply engine SHALL call `ApplyResourceChange`, extract the now-known computed
outputs, and persist them to the state store. State SHALL be written after each
successful apply so a failure mid-run leaves prior successes recorded.

#### Scenario: apply yields and persists deterministic outputs
- GIVEN a planned `alpha_token` with `label = "rec-X"` (counter 0)
- WHEN it is applied
- THEN state for that resource records `id = "alpha-0"` and `value = "alpha:rec-X:0"`.

#### Scenario: partial state persists on mid-run failure
- GIVEN A applies successfully and B then fails
- WHEN the run aborts
- THEN A's computed outputs remain in the state store.

### Requirement: Single-provider plan/apply integration against fakes
The change SHALL include an integration test that drives the real fake provider
binaries end-to-end (spawn, handshake, plan, apply) with no network, asserting
the persisted outputs equal the fakes' deterministic derivations.

#### Scenario: alpha end-to-end through the manager
- GIVEN an IR with one `alpha_token` (label "rec-X") and the alpha binary
- WHEN the executor plans and applies it through the plugin manager
- THEN the state store holds `id = "alpha-0"`, `value = "alpha:rec-X:0"`.
