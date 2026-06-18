# Spec delta: cli

## ADDED Requirements

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
