# Spec: e2e

## Purpose
The e2e capability is the milestone exit criterion made executable: the headline
two-provider round trip (docs/TESTING.md). It proves, against the real flake and
real fake provider binaries, that unknown values originating on both provider
sides resolve across ≥3 Nix-mediated phases to a fixpoint, that a Nix consumer
reads concrete outputs from both providers, that N>2 phases are required (not
incidental), and that a cyclic graph is rejected with an actionable error.
(destroy/refresh assertions are deferred to E3b.)
## Requirements
### Requirement: Headline two-provider round trip
The system SHALL pass an end-to-end test in which two providers produce unknown
values on both sides, resolution requires at least 3 apply phases reaching a
fixpoint, and a Nix consumer reads concrete outputs from both providers. The
test SHALL drive the real flake and real fake provider binaries.

#### Scenario: three-phase resolution to fixpoint
- GIVEN the flake topology A -> (Nix) -> B -> (Nix) -> C with a both-providers consumer
- WHEN the phase driver runs to a fixpoint
- THEN exactly 3 phases performed an apply and the loop halted at fixpoint (not a hardcoded count).

#### Scenario: final ledger and consumer are concrete
- WHEN the run completes
- THEN the ledger contains `A.id`, `A.value`, `B.endpoint`, and `C`'s outputs
- AND the `systemConfig` consumer's `recordEndpoint`, `tokenValue`, and `combined`
  are concrete and equal the deterministic provider derivations.

### Requirement: N>2 phases are required, not incidental
The headline test SHALL demonstrate that capping the loop at 2 phases leaves the
later resources and consumer unresolved.

#### Scenario: two-phase cap leaves work pending
- WHEN the driver runs with a 2-phase cap on the headline topology
- THEN `C` and the `combined` consumer value are not resolved and the run does not reach a clean fixpoint.

### Requirement: Cyclic dependency is rejected at fixpoint
The driver SHALL reach fixpoint with cyclic resources unapplied and return an
actionable error naming them, when the dependency graph is cyclic through Nix
(e.g. `A.label` depends on `C`).

#### Scenario: cycle variant names the stuck resources
- GIVEN a plan where `A.label` depends on `C.value` (cyclic with C depending on A)
- WHEN the driver runs
- THEN it returns an error identifying A and C as unresolvable (cycle / missing producer).

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

