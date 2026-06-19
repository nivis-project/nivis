# Spec: fake-providers

## Purpose
The in-repo fake providers are the hermetic, offline test substrate for every
integration and e2e test in Nivis (DESIGN D6). Two are required so the
milestone exit criterion can have unknown values originating on both provider
sides. They speak `tfprotov6` over go-plugin exactly as a real provider does,
producing computed-unknown-at-plan outputs that become known, deterministic
values at apply — giving the phased-eval loop real apply-time values to feed back
into Nix. The full provider/resource spec lives in `docs/TESTING.md`; this
capability records the behavioral requirements they must satisfy.
## Requirements
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
seedable via the `TERRAE_NIVIS_FAKE_COUNTER` environment variable (default 0). The
providers SHALL make no network calls, read no clock, and use no randomness.

#### Scenario: same inputs and seed yield identical outputs
- GIVEN two runs of `provider-alpha` with `TERRAE_NIVIS_FAKE_COUNTER=5` applying the same `alpha_token` config
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

### Requirement: A fake provider exercises collection and object types
There SHALL be a fake tfprotov6 provider whose resource has collection (list,
map) and nested-object attributes, both as inputs and as computed outputs, so the
value codec is exercised end-to-end through a real spawn / plan / apply. Outputs
SHALL be deterministic functions of the inputs and the seedable counter.

#### Scenario: collection/object computed attrs are unknown at plan
- GIVEN the `delta_thing` resource with `tags` and `ports` inputs set
- WHEN it is planned
- THEN its computed `endpoints` (list) and `meta` (object) are the tftypes
  unknown value, and `id` is unknown.

#### Scenario: collection/object values round-trip through apply
- GIVEN a planned `delta_thing` with known `tags`/`ports` inputs (counter 0)
- WHEN it is applied through the plugin manager
- THEN the decoded state contains a concrete `endpoints` list and a concrete
  `meta` object, equal to the provider's deterministic derivation, with no
  unknowns remaining.

#### Scenario: a map/list input is delivered to the provider
- GIVEN `tags = { env = "prod" }` and `ports = [80, 443]` in config
- WHEN the resource is applied
- THEN the provider receives those collection inputs (reflected in the
  deterministic outputs derived from them).

### Requirement: A fake provider serves a datasource
At least one in-repo fake provider SHALL declare a datasource type and implement
`ReadDataSource`, returning deterministic attributes computed from the request
config, so the datasource path (Nix declaration, IR, executor read, ledger,
dependent resource) is testable hermetically with no network and no credentials.
The datasource's returned attributes SHALL be deterministic given the same config
(per the existing hermetic-outputs requirement), so tests can assert exact values.

#### Scenario: the fake returns deterministic datasource attributes
- GIVEN the fake provider's datasource type read with a given config
- WHEN ReadDataSource is called twice with the same config
- THEN both calls return identical attributes.

#### Scenario: a datasource output feeds a resource end to end
- GIVEN an IR with the fake's datasource and a resource whose config references a datasource output
- WHEN the phased loop runs against the fake binary
- THEN the datasource is read, its output resolves the resource's ref, and the resource applies with the datasource-derived value.

### Requirement: A fake provider that rejects an unconfigured Configure
There SHALL be an in-repo fake provider whose `ConfigureProvider` returns an error
diagnostic when configured with a null/empty config (mimicking a credential-
requiring real provider such as proxmox), while still serving `GetProviderSchema`
normally. It exists so the "schema fetch skips configure" behaviour is provable
hermetically (no network, no real credentials): the existing fakes' configure
always succeeds, so they cannot catch a configure-before-schema regression. The
fake SHALL be available on the `#fake-providers` PATH (or buildable in the e2e).

#### Scenario: configure fails but schema is still served
- GIVEN the configure-rejecting fake spawned
- WHEN it is configured with an all-null config
- THEN configure returns an error diagnostic
- AND GetProviderSchema still returns the provider's resource schema.

