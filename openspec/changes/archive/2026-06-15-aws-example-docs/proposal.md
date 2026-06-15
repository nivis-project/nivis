# Proposal: aws-example-docs

## Why
The real-provider cycle (registry fetch → v5 negotiate → configure → plan →
apply → destroy against AWS) is proven, but only in gated Go tests — a user
**cannot reproduce it through the `tn` CLI**: there is no Nix example declaring a
real provider, and the docs still say real providers are "out of scope /
network-gated" (now stale). This change makes the real cycle user-runnable and
documents it, so "terrae nivis drives real providers" is demonstrable, not just
asserted in tests.

## What changes
- Add `nix/example/aws.nix` and expose it as the flake attr `terraeNivis.aws`: a
  `ledger`-function (same shape as `terraeNivis.plan`) declaring provider
  `aws` with `source = "registry.opentofu.org/hashicorp/aws"` and one
  `aws_s3_bucket` resource with `force_destroy = true` and a `nixform-test` tag,
  `bucket` omitted (AWS generates a unique name → safe, no collision).
- A **"Real providers (AWS)"** section in `README.md` + `docs/GETTING-STARTED.md`
  showing the CLI flow: `AWS_PROFILE=… AWS_REGION=… tn apply --attr
  terraeNivis.aws` → `tn state show …` → `tn destroy --attr terraeNivis.aws`,
  with the explicit note that **this creates a real (free-tier) S3 bucket** and
  destroys it, and that **credentials/region come from the environment** (the AWS
  SDK default chain) since Nix-side provider config is a separate item
  (beans-prj4).
- Replace the stale "real provider support is out of PoC scope / network-gated"
  disclaimers in README/getting-started with an accurate statement (real
  plan/apply/destroy works; registry download + checksum verification; v5/v6).

## Non-goals
- Nix-side provider config (region/profile in the config) — still env-based;
  tracked as beans-prj4. The docs note this.
- A gated automated CLI e2e for AWS in CI — the existing gated Go tests already
  prove the path; this change is the user-facing example + docs. (I will run the
  documented `tn` flow once manually to verify it before documenting it.)
- Hetzner / other providers — AWS is the documented example; others work the same
  way by changing the source + resource type.

## Impact
- New: `nix/example/aws.nix`, flake `terraeNivis.aws`; README + getting-started
  gain a real-AWS section; stale disclaimers corrected. No Go/executor changes.
- Verification: the documented `tn apply/state/destroy --attr terraeNivis.aws`
  flow is run once against real AWS (creating then destroying one bucket) to
  confirm the docs are accurate; no orphaned resources.
