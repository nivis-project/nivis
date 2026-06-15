# Spec delta: e2e

## ADDED Requirements

### Requirement: A real-provider (AWS) example is runnable via the CLI
The flake SHALL expose an attribute (`terraeNivis.aws`) that declares a real
provider (`hashicorp/aws`) and a resource, such that `tn apply`/`state`/`destroy`
`--attr terraeNivis.aws` drives the real provider end to end (registry fetch,
configure, plan, apply, destroy), with credentials and region taken from the
environment.

#### Scenario: real AWS apply/destroy via tn
- GIVEN valid AWS credentials in the environment (e.g. AWS_PROFILE + AWS_REGION)
- WHEN the user runs `tn apply --attr terraeNivis.aws` then `tn destroy --attr terraeNivis.aws`
- THEN a real S3 bucket is created (AWS-generated name, force_destroy) and then
  destroyed, with no resource left behind.

### Requirement: Real-provider support is documented accurately
The README and getting-started docs SHALL document the real-provider (AWS) CLI
flow and SHALL NOT state that real providers are out of scope. They SHALL note
that the flow creates a real resource and that credentials/region come from the
environment.

#### Scenario: docs show the real-AWS flow
- WHEN a user reads the docs
- THEN they find a "Real providers (AWS)" section with the `tn apply/state/destroy
  --attr terraeNivis.aws` commands, the real-resource warning, and the
  environment-credentials note — and no stale "out of scope" disclaimer.
