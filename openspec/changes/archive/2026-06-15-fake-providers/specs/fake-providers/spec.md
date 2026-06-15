# Spec delta: fake-providers

## ADDED Requirements

### Requirement: Providers speak tfprotov6 over go-plugin
Each fake provider SHALL be a standalone Go binary that serves the
`tfprotov6.ProviderServer` interface via the `terraform-plugin-go`
`tf6server.Serve` helper, completing the standard go-plugin/gRPC handshake so an
unmodified protocol client can drive it.

#### Scenario: provider serves a schema over the protocol
- GIVEN a built `provider-alpha` binary
- WHEN a `tfprotov6` client calls `GetProviderSchema`
- THEN it returns a schema declaring the `alpha_token` resource type without error.

#### Scenario: unneeded RPCs are safely unimplemented
- GIVEN a fake provider
- WHEN a client calls a data-source, function, or ephemeral-resource RPC
- THEN the provider returns an empty/unimplemented response with a diagnostic, not a crash.

### Requirement: alpha_token resource schema and computed outputs
`provider-alpha` SHALL expose resource `alpha_token` with an optional string
input `label` and two computed string attributes `id` and `value` that are
unknown at plan and known after apply.

#### Scenario: computed attrs are unknown at plan
- GIVEN an `alpha_token` config with `label` known (or null)
- WHEN `PlanResourceChange` is called
- THEN the planned state has `id` and `value` set to the tftypes unknown value.

#### Scenario: computed attrs become known and deterministic at apply
- GIVEN a planned `alpha_token` with `label = "rec-X"`
- WHEN `ApplyResourceChange` is called with per-process counter seeded to 0
- THEN `NewState` contains no unknown values
- AND `value` equals `"alpha:" + label + ":" + counter` (here `"alpha:rec-X:0"`)
- AND `id` equals `"alpha-" + counter` (here `"alpha-0"`)
- AND the counter increments by 1 per applied resource in the process.

#### Scenario: label may be absent
- GIVEN an `alpha_token` config with `label` null (the e2e's resource A)
- WHEN it is applied with counter 0
- THEN `value` equals `"alpha::0"` (empty label segment) and `id` equals `"alpha-0"`, with no error.

### Requirement: beta_record resource schema and computed output
`provider-beta` SHALL expose resource `beta_record` with a required string input
`from` and one computed string attribute `endpoint` that is unknown at plan and
known after apply.

#### Scenario: required input is validated
- GIVEN a `beta_record` config with `from` null
- WHEN `ValidateResourceConfig` (or apply) runs
- THEN a diagnostic reports that `from` is required.

#### Scenario: endpoint is unknown at plan, deterministic at apply
- GIVEN a planned `beta_record` with `from = "rec-alpha:rec-X:0"`
- WHEN `PlanResourceChange` is called
- THEN `endpoint` is the tftypes unknown value
- AND WHEN `ApplyResourceChange` is called
- THEN `endpoint` equals `"beta://" + from` (here `"beta://rec-alpha:rec-X:0"`) with no unknown values remaining.

### Requirement: Deterministic, hermetic outputs
Provider outputs SHALL be a pure function of inputs and a per-process counter
seedable via the `NIXFORM_FAKE_COUNTER` environment variable (default 0). The
providers SHALL make no network calls, read no clock, and use no randomness.

#### Scenario: same inputs and seed yield identical outputs
- GIVEN two runs of `provider-alpha` with `NIXFORM_FAKE_COUNTER=5` applying the same `alpha_token` config
- WHEN both produce `NewState`
- THEN the `id` and `value` are byte-identical across runs.

### Requirement: In-process protocol conformance test
The change SHALL include a Go test that drives each provider's
`tfprotov6.ProviderServer` directly (in-process, no network) through
`GetProviderSchema` -> `PlanResourceChange` -> `ApplyResourceChange`, asserting
the unknown-at-plan / known-at-apply transition and the exact derived values.

#### Scenario: alpha plan/apply round trip asserts exact values
- GIVEN the alpha provider server with counter seeded to 0
- WHEN the test plans then applies an `alpha_token` with `label = "rec-X"`
- THEN plan yields unknown `id`/`value` and apply yields `id = "alpha-0"`, `value = "alpha:rec-X:0"`.
