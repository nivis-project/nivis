# Proposal: provider-configure

## Why
Real providers must be configured (`ConfigureProvider` in v6 / `Configure` in v5)
before any plan or apply — the AWS provider crashes with `EOF` if you plan
without configuring (confirmed by probe). The executor currently spawns a
provider and goes straight to plan, which works only for the trivially
config-free fakes. This change adds provider configuration so real providers
(AWS, Hetzner) can be planned, and wires provider `config` from the IR through to
`Configure`. It is the last gap before a real, read-only AWS plan.

## What changes
- Add `Configure(ctx, ResourceSchema-less provider config)` to the neutral
  `provider.Client` interface, implemented by both the v5 and v6 backends:
  fetch the provider config schema (already cached), encode the given config map
  (all-absent attributes -> null, so the provider falls back to its own default
  resolution, e.g. the AWS SDK credential/region chain), and call the protocol's
  configure RPC. Surface diagnostics as errors.
- The plugin manager calls `Configure` once per spawned provider, using the
  provider's `config` from the IR (`providers.<id>.config`), before returning the
  client for plan/apply. Configuration happens once and is remembered (the
  process is pooled).
- A real, network+credential-gated test (NIXFORM_NET_TESTS=1 + AWS creds via
  AWS_PROFILE) that configures the real AWS provider and PLANS `aws_s3_bucket`
  (no required inputs; no resource created), asserting a planned state comes back
  with no error.

## Non-goals
- Real `apply`/create against AWS — read-only plan only here; apply is a separate,
  explicitly-authorized step.
- SDKv2 nested blocks in the schema model (beans-9qrj) — `aws_s3_bucket` plans
  fine with only its flat attributes for this probe.
- Rich provider-config authoring in Nix — for now the IR's `providers.<id>.config`
  (possibly empty) is passed through; AWS resolves region/creds from the
  environment (AWS_PROFILE/AWS_REGION) via its default chain.

## Impact
- Changed: `internal/provider` (interface + v5/v6 Configure), `internal/plugin`
  (configure-on-spawn using IR provider config).
- New: a gated real-AWS plan test.
- Unblocks driving real providers end to end through plan; closes the
  configure gap. The AWS plan validates the whole stack (registry -> v5 negotiate
  -> configure -> schema -> collections codec -> plan) against a real cloud,
  read-only.
