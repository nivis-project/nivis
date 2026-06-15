# Spec: cli

## Purpose
The `nixform` CLI is the user-facing surface of the executor. Beyond the
commands themselves (plan/apply/destroy/refresh/state — see the executor spec),
this capability governs error presentation: failures must be actionable and carry
the identity of the offending entity, never a raw stack trace or an unhelpful
dump. A newcomer should be able to read an error and know what to fix.
## Requirements
### Requirement: Errors are actionable, never raw dumps
The CLI SHALL print runtime failures as a single `error: <message>` line to
stderr and exit non-zero, without dumping command usage text or a Go stack
trace. Command usage SHALL be shown only for flag/argument misuse.

#### Scenario: runtime error prints a clean message
- GIVEN a command that fails at runtime (e.g. an unknown flake attribute)
- WHEN it runs
- THEN stderr contains an `error:` line describing the cause and the process
  exits non-zero, and the command's usage block is NOT printed.

#### Scenario: flag misuse still shows usage
- GIVEN an unknown flag
- WHEN a command runs
- THEN usage is shown (this is flag misuse, not a runtime error).

### Requirement: Nix evaluation failures surface the real cause
When `nix eval` fails, the CLI SHALL surface the Nix `error:` lines from stderr
and SHALL omit non-actionable noise such as the "Git tree is dirty" warning.

#### Scenario: nix error is extracted
- GIVEN a flake evaluation that fails with a Nix `error:`
- WHEN the driver reports it
- THEN the message includes the Nix error text and excludes the dirty-tree warning.

### Requirement: Error classes carry identity
Each error class SHALL name the relevant entity: IR/schema validation names the
offending resource/edge/path; provider diagnostics include the provider's
summary and detail; state errors name the state path; phase fixpoint failures
name the stuck resource(s) and awaited inputs.

#### Scenario: provider diagnostic surfaces summary and detail
- GIVEN a provider returns an error diagnostic
- WHEN the operation reports it
- THEN the message contains the diagnostic's summary and detail.

