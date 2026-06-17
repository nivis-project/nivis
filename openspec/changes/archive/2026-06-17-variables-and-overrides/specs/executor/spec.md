# Spec delta: executor

## ADDED Requirements

### Requirement: Resolve and inject variables with Terraform precedence
The executor SHALL resolve a variable map before evaluation and inject it as the
ledger `vars` object on every phase. Values SHALL be resolved from these sources,
**lowest to highest priority**, so a later source overrides an earlier one for the
same name:

1. environment variables named `NIVIS_VAR_<name>`;
2. `--var-file <path>` (a JSON object of name to value); when given multiple
   times, a later file overrides an earlier one;
3. `--var name=value` flags; when given multiple times, a later flag overrides an
   earlier one.

Declared defaults are NOT applied by the executor; they are the Nix layer's job
(`mkVars` fills an unset variable from its default). The executor only collects
the externally-supplied values above. The resolved `vars` map SHALL be the same
for every phase of the run. A malformed `--var` (no `=`) or an unreadable or
non-object `--var-file` SHALL produce an actionable error that names the offending
input. Variable values SHALL travel only in the 0600 ledger file, never on the Nix
command line, preserving the existing purity and secret-handling guarantees.

#### Scenario: an explicit flag overrides env and file
- GIVEN `NIVIS_VAR_region=eu-west-1`, a var-file setting `region=eu-central-1`, and `--var region=us-east-1`
- WHEN the executor resolves variables
- THEN `vars.region` is `us-east-1` (the flag wins).

#### Scenario: a var-file overrides the environment
- GIVEN `NIVIS_VAR_region=eu-west-1` and a var-file setting `region=eu-central-1`, with no `--var region`
- WHEN the executor resolves variables
- THEN `vars.region` is `eu-central-1` (the file beats env).

#### Scenario: later flags and files override earlier ones
- GIVEN `--var x=1 --var x=2` (and, separately, two `--var-file`s both setting `y`)
- WHEN the executor resolves variables
- THEN `x` is `2`, and `y` is the value from the later file.

#### Scenario: a malformed input is an actionable error
- GIVEN `--var notanassignment` (no `=`) or a `--var-file` that is not a readable JSON object
- WHEN the executor resolves variables
- THEN it fails with an error naming the offending flag or file, not a stack trace.

#### Scenario: variables are injected on every phase
- GIVEN a run that takes more than one phase and has variables set
- WHEN each phase is evaluated
- THEN the same resolved `vars` map is present in the injected ledger every phase.
