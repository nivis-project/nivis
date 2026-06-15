# Spec delta: executor

## ADDED Requirements

### Requirement: The executor realises __build leaves before apply
Before applying a resource, the executor SHALL walk that resource's resolved
config for `__build` leaves and **realise** each one's store path that is not
already valid (building the derivation, e.g. via `nix-store --realise` on the
store root), then substitute the concrete path into the config sent to the
provider. This happens **per resource, as it becomes ready**, so across the phased
loop only the builds reachable from the resources ready in a given phase are
performed — never "everything" — and a build whose inputs come from an earlier
resource's outputs (a Nix expression that consumes a prior apply's value) is
realised in the later phase once it is evaluable. An author opt-out (`--no-build`)
SHALL skip realising (assuming paths are already built). A realise failure SHALL
produce an error naming the store path.

#### Scenario: a build output is realised before the provider reads it
- GIVEN a resource whose config has a `__build` leaf for an unbuilt store path
- WHEN that resource is applied
- THEN the executor realises the path first, and the provider receives the concrete (now-existing) path — no "no such file or directory".

#### Scenario: only what is ready this phase is built
- GIVEN resource B's `__build` derivation depends on resource A's apply-time output
- WHEN the loop runs
- THEN A is applied first, the config re-evaluates so B's derivation becomes known, and B's build is realised in the later phase (not before A) — the build participates in the fixpoint.

#### Scenario: --no-build skips realising
- GIVEN `--no-build` and a resource with a `__build` leaf
- WHEN it is applied
- THEN the executor does not build; it uses the path as-is (and the provider errors if it is missing — the user opted out).
