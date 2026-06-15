# Spec delta: e2e

## ADDED Requirements

### Requirement: Installing the tn CLI is documented
The docs SHALL include a standalone install guide (`docs/INSTALL.md`) covering how
to obtain the `tn` CLI without a repo checkout — at minimum `nix run
github:wearetechnative/terrae-nivis#tn`, an ad-hoc `nix shell`, a persistent
install (`nix profile install`), and building from a clone (`go`/`nix build`). It
SHALL note `tn`'s runtime needs (Nix available; network for the first provider
fetch). Tutorials SHALL link to it rather than repeating install steps, and the
docs-SSOT check SHALL treat it as the canonical install instructions.

#### Scenario: the install guide is discoverable
- WHEN the docs site is built
- THEN an "Install" page exists (including `docs/INSTALL.md`) and is reachable
  from the nav, and the AWS tutorial links to it.

## MODIFIED Requirements

### Requirement: A from-scratch AWS S3 tutorial exists and is verified
The docs SHALL include a genuinely from-scratch tutorial
(`docs/TUTORIAL-AWS-S3.md`) that starts from an **empty directory on the user's
own machine** — not the terrae-nivis repo. It SHALL: (1) install `tn` (linking
`docs/INSTALL.md`); (2) scaffold a fresh infra flake (`nix flake init`) whose
`flake.nix` takes `terrae-nivis` as a flake input, uses its `lib`
(`mkResource`/`mkProvider`/`toIR`), and exposes `terraeNivis.plan`; (3) add the
AWS S3 resource (explained); then run `plan`/`apply`/state inspection/`destroy`
from the user's own flake directory, with troubleshooting. The tutorial's commands
and outputs SHALL reflect runs actually performed (the fresh-flake `plan` and the
real-AWS `apply`/`destroy`, no resource left behind). It SHALL remain the single
canonical long-form AWS walkthrough: getting-started §7 links to it, and the
docs-SSOT check SHALL guard against duplication.

#### Scenario: the tutorial is discoverable and renders
- WHEN the docs site is built
- THEN a "Tutorial: an S3 bucket on AWS" page exists (including
  `docs/TUTORIAL-AWS-S3.md`) and is reachable from the nav.

#### Scenario: a fresh consuming flake works
- GIVEN an empty directory with a `flake.nix` that inputs terrae-nivis and exposes `terraeNivis.plan`
- WHEN `tn plan` is run in that directory
- THEN it evaluates the flake and reports the planned resource (no repo checkout required).

#### Scenario: the walkthrough is not duplicated
- WHEN the docs-SSOT check runs
- THEN the AWS plan/apply/state/destroy walkthrough and the install steps each
  appear in exactly one canonical location, and getting-started §7 links to the
  tutorial rather than restating it.
