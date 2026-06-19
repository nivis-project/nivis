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
Apply output SHALL render each applied node by the **operation it actually
resolved as** (the op the phase driver reports for that node), so an in-place
update shows `~`, a replacement shows `-/+`, a no-op shows `=`, and only a real
create shows `+`; it SHALL NOT render every applied node as a create.

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

#### Scenario: apply shows the real op, not always create
- GIVEN a second apply of an already-applied, unchanged stack
- WHEN it prints
- THEN each resource is shown with its true op (a no-op `=` or update `~`), not `+` create.

### Requirement: Shell completion with dynamic resource-id completion
The CLI SHALL provide a `completion` command that prints a shell-completion
script for `bash`, `zsh`, `fish`, and `powershell`. It SHALL dynamically complete
resource ids from the local state file: the positional argument of `state show`
and `state rm`, and the value of the `--target` flag, SHALL complete to the ids
present in state. When the state file is missing or unreadable, completion SHALL
return no suggestions (and SHALL NOT fall back to filename completion). Completion
is presentation only: it performs no apply, plan, or destroy and reads only the
state file.

#### Scenario: completion scripts are available per shell
- GIVEN the CLI
- WHEN `nivis completion bash` (or zsh/fish/powershell) is run
- THEN it prints a valid completion script for that shell.

#### Scenario: state ids complete from the state file
- GIVEN a state file containing several resource ids
- WHEN completion is requested for the `state show` argument or the `--target` flag
- THEN the suggestions are exactly the ids in state.

#### Scenario: no state file yields no completions, not filenames
- GIVEN no state file (or an unreadable one)
- WHEN completion is requested for a resource id
- THEN no suggestions are returned and the shell does not fall back to filename completion.

### Requirement: Move state with pull and push, guarded against accidental loss
The CLI SHALL provide `state pull` and `state push`. `state pull` SHALL write the
whole state document (the backend's snapshot) to stdout, or to a file with
`--out`. `state push` SHALL replace the whole state from stdin, or from a file
with `--in`, after the input parses as a valid state document. Because `push`
overwrites the state of record, it SHALL require confirmation before replacing
existing state: it SHALL report the resource counts (incoming vs current) and
prompt, and SHALL proceed without prompting only when `--force` (equivalently
`--yes`) is given. When the input is not an interactive terminal (a pipe or
redirect), `push` SHALL require `--force` and SHALL refuse otherwise with a clear
message, so a scripted push is always explicit.

#### Scenario: pull writes the snapshot
- GIVEN state with several resources
- WHEN `nivis state pull` runs
- THEN the whole state document is written to stdout (or the `--out` file), and re-pushing it reproduces the same state.

#### Scenario: push replaces state after confirmation
- GIVEN a valid state document on stdin and `--force`
- WHEN `nivis state push` runs
- THEN the state is replaced by the document's resources.

#### Scenario: a non-interactive push without --force is refused
- GIVEN `state push` reading from a pipe (non-TTY) without `--force`
- WHEN it runs
- THEN it refuses with a message telling the user to pass `--force`, and state is unchanged.

#### Scenario: push rejects malformed input
- GIVEN `state push` given input that is not a valid state document
- WHEN it runs
- THEN it fails with an error and leaves the existing state unchanged.

### Requirement: State commands give clear messages on missing ids and empty state
`state show` and `state rm` SHALL give a clear message when the named id is not in
state (rather than failing obscurely or silently succeeding), and `state list`
SHALL indicate when state is empty.

#### Scenario: rm of a missing id is reported
- GIVEN an id not present in state
- WHEN `nivis state rm <id>` runs
- THEN it reports that the id was not in state (not a silent success).

#### Scenario: list of empty state says so
- GIVEN empty state
- WHEN `nivis state list` runs
- THEN it indicates there are no resources in state.

### Requirement: Read stack outputs with the output command
The CLI SHALL provide `nivis output [name]`: with no argument it prints all
declared outputs as human-readable `name = value` lines; with a `name` it prints
just that output's value (erroring with a clear message if the name is not a
declared output). A `--json` flag SHALL print a JSON object of `{ name: value }`
(or the single value with a name argument) for machine consumption. Color/TTY
handling SHALL match the rest of the CLI (no ANSI when piped or `NO_COLOR`). A
sensitive output SHALL be redacted in human output unless explicitly revealed, and
SHALL never be written to a world-readable location by this command.

#### Scenario: output prints all declared outputs
- GIVEN an applied stack declaring `ip` and `url`
- WHEN `nivis output` runs
- THEN it prints `ip = <value>` and `url = <value>`.

#### Scenario: output of a single name
- GIVEN the same stack
- WHEN `nivis output url` runs
- THEN it prints just the resolved value of `url`.

#### Scenario: json form for machine consumption
- GIVEN the same stack
- WHEN `nivis output --json` runs
- THEN it prints a JSON object mapping each output name to its resolved value.

#### Scenario: an unknown output name errors
- GIVEN a stack with no output named `nope`
- WHEN `nivis output nope` runs
- THEN it fails with a message that `nope` is not a declared output.

### Requirement: nivistutor scaffolds a tutorial's starter files
The project SHALL provide `nivistutor`, distributed as the flake app `#tutor`,
that scaffolds a chosen tutorial's starter files into the user's filesystem so the
user can read, edit, and run them with plain `nivis`. It SHALL:
- greet the user and present the available tutorials to choose from (at least a
  from-scratch **getting-started** tutorial and the current release's **features**
  tutorial);
- ask whether to write into a **new subdirectory** or the **current directory**;
- write that tutorial's starter directory verbatim: a working `flake.nix` (a
  `nivis` input and a `nivis.plan`), the config, and a `README.md` of next steps;
- print the next steps (the exact `nivis …` commands to run) and SHALL NOT run
  `nivis` itself.
The starter files SHALL be embedded in the binary (so `nivistutor` works offline
and writes files matching its own build), and SHALL NOT be fetched over the
network. `nivistutor` SHALL refuse to overwrite existing files without explicit
confirmation. A non-interactive mode (a flag selecting the tutorial and target
dir) SHALL exist so the behaviour is testable.

#### Scenario: scaffold a chosen tutorial into a new directory
- GIVEN `nivistutor` with a tutorial selected and a new target directory
- WHEN it runs
- THEN the tutorial's `flake.nix`, config, and `README.md` are written there, and it prints the `nivis` commands to run next, without running nivis.

#### Scenario: the menu lists more than one tutorial
- GIVEN the available tutorials
- WHEN `nivistutor` starts
- THEN it offers at least the getting-started tutorial and the current release's features tutorial.

#### Scenario: it does not clobber existing files silently
- GIVEN a target directory already containing a tutorial file
- WHEN `nivistutor` would write over it
- THEN it does not overwrite without explicit confirmation.

#### Scenario: scaffolded starter runs with plain nivis
- GIVEN a scaffolded tutorial directory and the fake providers on PATH (via `nix shell …#tutor`)
- WHEN the user runs the documented `nivis plan` from that directory
- THEN it evaluates the starter's `nivis.plan` with no `--flake`/`--attr` flags.

