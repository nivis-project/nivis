# Spec delta: e2e

## MODIFIED Requirements

### Requirement: A real-provider (AWS) example is runnable via the CLI
The flake SHALL expose an attribute (`terraeNivis.aws`) that declares a real
provider (`hashicorp/aws`) and a resource, such that `tn apply`/`state`/`destroy`
`--attr terraeNivis.aws` drives the real provider end to end (registry fetch,
configure, plan, apply, destroy). The provider's **region** (and other non-secret
settings) SHALL be expressed in the Nix config (via `mkProvider`) and flow into
`Configure`; only credentials are taken from the environment (the AWS SDK default
chain).

#### Scenario: real AWS apply/destroy via tn
- GIVEN valid AWS credentials in the environment (e.g. AWS_PROFILE) and a region declared in the Nix provider config
- WHEN the user runs `tn apply --attr terraeNivis.aws` then `tn destroy --attr terraeNivis.aws`
- THEN a real S3 bucket is created (AWS-generated name, force_destroy) and then
  destroyed, with no resource left behind.

### Requirement: Real-provider support is documented accurately
The README and getting-started docs SHALL document the real-provider (AWS) CLI
flow and SHALL NOT state that real providers are out of scope. They SHALL note
that the flow creates a real resource, that provider settings such as region are
expressed in the Nix config, and that credentials come from the environment.

#### Scenario: docs show the real-AWS flow
- WHEN a user reads the docs
- THEN they find a "Real providers (AWS)" section with the `tn apply/state/destroy
  --attr terraeNivis.aws` commands, the real-resource warning, the Nix-side
  provider config (region), and the environment-credentials note — and no stale
  "out of scope" disclaimer.
