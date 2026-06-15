# Spec delta: e2e

## ADDED Requirements

### Requirement: A from-scratch AWS S3 tutorial exists and is verified
The docs SHALL include a from-scratch, step-by-step tutorial
(`docs/TUTORIAL-AWS-S3.md`) that walks a newcomer through creating and destroying
a real AWS S3 bucket with `tn` — prerequisites, obtaining `tn`, AWS credentials,
the Nix config (explained), `plan`/`apply`/state inspection/`destroy`, and
troubleshooting. The tutorial's commands and outputs SHALL reflect a run actually
performed against real AWS (create then destroy, no resource left behind). The
tutorial SHALL be the single canonical long-form AWS walkthrough: getting-started
§7 links to it rather than repeating the command set, and the docs-SSOT check
SHALL guard against duplication.

#### Scenario: the tutorial is discoverable and renders
- WHEN the docs site is built
- THEN a "Tutorial: an S3 bucket on AWS" page exists (including
  `docs/TUTORIAL-AWS-S3.md`) and is reachable from the nav.

#### Scenario: the walkthrough is not duplicated
- WHEN the docs-SSOT check runs
- THEN the AWS `tn apply/state/destroy --attr terraeNivis.aws` walkthrough appears
  in exactly one canonical location, and getting-started §7 links to it rather
  than restating it.
