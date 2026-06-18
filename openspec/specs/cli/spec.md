# Spec: cli

## Purpose
The `Nivis` CLI is the user-facing surface of the executor. Beyond the
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

### Requirement: Plan and apply output is colorised by change type and grouped by phase
The CLI SHALL render plan, apply, and destroy output with the change type shown by
a marker and, on a color-capable terminal, a color: `+` create (green), `~` update
(yellow), `-/+` replace (red+green), `-` destroy (red), `=` no-op (dim), and a
datasource read with a distinct `r` marker in a read (dim) color so a read is
visibly not a create. Apply output SHALL group applied nodes under the phase they
resolved in, so the phased fixpoint is visible, and SHALL retain a count summary.

Color SHALL be gated exactly as the existing `colorEnabled` helper specifies: when
the output writer is not a TTY (e.g. piped or redirected) or when `NO_COLOR` is
set, the output SHALL contain **no ANSI escape codes**, while the markers, the
text, the counts, and the phase grouping SHALL be identical to the colored form
(so scripts and tests see stable output). The output SHALL be written through the
command's writer (capturable), not the process stdout directly.

#### Scenario: change types are colored on a TTY
- GIVEN a plan with a create, an update, a replace, and a no-op, rendered to a color-capable writer
- WHEN it is printed
- THEN each line carries its marker and an ANSI color appropriate to its change type.

#### Scenario: no color when piped or NO_COLOR is set
- GIVEN the same plan rendered to a non-TTY writer, or with `NO_COLOR` set
- WHEN it is printed
- THEN the output contains no ANSI escape codes, but the same markers, text, counts, and phase grouping as the colored form.

#### Scenario: apply output is grouped by phase
- GIVEN an apply that resolved resources across multiple phases
- WHEN it prints
- THEN resources are listed under their phase heading in phase order, with a summary of the total applied and the phase count.

#### Scenario: a datasource read is shown distinctly
- GIVEN an apply that read a datasource and created a resource
- WHEN it prints
- THEN the datasource line uses the `r` read marker (and read color on a TTY), distinct from the `+` create marker.

