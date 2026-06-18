# Spec delta: cli

## ADDED Requirements

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
