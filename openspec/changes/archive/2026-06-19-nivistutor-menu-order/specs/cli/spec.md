# Spec delta: cli

## MODIFIED Requirements

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

The tutorial list (both `--list` and the interactive menu) SHALL be presented in a
deterministic, controlled order, not merely alphabetical: the **getting-started**
tutorial SHALL always be listed first (it is the entry point), followed by the
`features-<version>` tutorials ordered **newest version first** (compared
numerically by version), followed by any other tutorials alphabetically. A new
`features-<version>` tutorial SHALL slot into this order by its version without a
code change.

#### Scenario: scaffold a chosen tutorial into a new directory
- GIVEN `nivistutor` with a tutorial selected and a new target directory
- WHEN it runs
- THEN the tutorial's `flake.nix`, config, and `README.md` are written there, and it prints the `nivis` commands to run next, without running nivis.

#### Scenario: the menu lists more than one tutorial
- GIVEN the available tutorials
- WHEN `nivistutor` starts
- THEN it offers at least the getting-started tutorial and the current release's features tutorial.

#### Scenario: getting-started is listed first, then features newest-first
- GIVEN the embedded tutorials getting-started and several features-<version> (e.g. 0.4, 0.5, 0.10)
- WHEN the list or menu is presented
- THEN getting-started is first, and the features tutorials follow it ordered newest-version-first (0.10 before 0.5 before 0.4, a numeric not lexical compare).

#### Scenario: it does not clobber existing files silently
- GIVEN a target directory already containing a tutorial file
- WHEN `nivistutor` would write over it
- THEN it does not overwrite without explicit confirmation.

#### Scenario: scaffolded starter runs with plain nivis
- GIVEN a scaffolded tutorial directory and the fake providers on PATH (via `nix shell …#tutor`)
- WHEN the user runs the documented `nivis plan` from that directory
- THEN it evaluates the starter's `nivis.plan` with no `--flake`/`--attr` flags.
