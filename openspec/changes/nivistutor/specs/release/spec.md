# Spec delta: release

## ADDED Requirements

### Requirement: Tutorials are self-contained, versioned starter directories
Each tutorial SHALL exist as a self-contained starter directory under
`nix/example/tutorial-<name>/`, containing a working `flake.nix` (a `nivis` input
and a `nivis.plan`), its config, and a `README.md` of next steps, so it can be
scaffolded out (by `nivistutor`) and run with plain `nivis`. Per-release feature
tutorials SHALL be named by version (`tutorial-features-<version>`), while the
from-scratch entry tutorial (`tutorial-getting-started`) is a single,
continuously-updated starter. The flake SHALL expose each tutorial's config so the
repo's own checks (the milestone-notes golden gate, the docs includes) can
reference it.

#### Scenario: a tutorial is a runnable starter directory
- GIVEN a tutorial starter directory
- WHEN its files are copied elsewhere and `nivis plan` is run from there (with the fake providers on PATH)
- THEN it evaluates without `--flake`/`--attr` flags.

#### Scenario: release feature tutorials are version-named
- GIVEN a release's feature tutorial
- WHEN it is added
- THEN it lives at `nix/example/tutorial-features-<version>/`, distinct from prior releases' tutorials.
