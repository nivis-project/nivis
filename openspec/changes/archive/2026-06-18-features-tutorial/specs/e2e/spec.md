# Spec delta: e2e

## ADDED Requirements

### Requirement: A hands-on feature tutorial runs against the fakes
There SHALL be a tutorial, runnable against the in-repo fake providers with no
cloud, credentials, or network, that exercises the daily-driver features in one
config: variables (required + a default, set via `--var`), a datasource feeding a
resource, the round trip across phases, declared stack outputs, and the
phase-grouped plan/apply output. It SHALL be backed by a bundled config exposed as
a flake attribute so the documented commands run as written, and its outputs SHALL
be deterministic (the fakes are hermetic) so they match the tutorial text.

#### Scenario: the tutorial config applies and resolves outputs against the fakes
- GIVEN the bundled tutorial config applied against the fake providers with the required variable set
- WHEN it is applied to a fixpoint and outputs are resolved
- THEN the datasource is read in the first phase, the round-trip resource resolves in a later phase, and the declared outputs (including a datasource-derived one and the round-trip value) are concrete.
