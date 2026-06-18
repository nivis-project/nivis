# Proposal: shell-completion

## Why
The CLI has no tab-completion: a user types `nivis state show ` and gets nothing,
and there is no `nivis completion` to install shell support. Completion is a
basic daily-driver affordance (A4 of the "Road to v1" milestone, beans-oycy).
Cobra provides the generator and the dynamic-completion hooks; we just wire them
up, and add the one Nivis-specific completer that matters: resource ids from the
state file.

## What changes
- **`nivis completion <bash|zsh|fish|powershell>`** prints a completion script for
  the shell, via Cobra's built-in generator (it is auto-registered; this makes it
  explicit and documented, not hidden/disabled).
- **Dynamic resource-id completion** from the local state file:
  - the `state show` and `state rm` positional argument completes to the ids in
    state (`ValidArgsFunction`),
  - the persistent `--target` flag completes to the same (`RegisterFlagCompletionFunc`).
  A completer reads `store.List()` and returns the ids; on any error (no state
  file yet, unreadable) it returns no completions and `NoFileComp` so the shell
  does not fall back to filename completion.
- **Docs:** an "install completion" section in `docs/INSTALL.md` (the per-shell
  one-liners).

This is pure DX: no command behaviour, no IR, nothing applied differently.

## Non-goals
- Completing resource *types*, provider names, or attribute names (would need a
  provider schema / a flake eval at completion time; out of scope, and slow).
- Completing flag *values* other than `--target` (e.g. `--flake` paths get
  Cobra's default file completion, which is correct).
- Bundling/installing the completion script automatically; the user runs the
  documented one-liner for their shell.

## Impact
- CLI: `cmd/nivis` gains a small `stateIDs` completer and wires it to `state
  show`/`state rm` (`ValidArgsFunction`) and `--target`
  (`RegisterFlagCompletionFunc`). The `completion` command is Cobra's built-in.
- Docs: `docs/INSTALL.md` gains a completion section.
- Tests: `cmd/nivis` unit test that the completer returns the ids in a populated
  store and an empty list (with `NoFileComp`) for a missing/empty store.

Docs impact: new paragraph/section, an "install completion" section in
docs/INSTALL.md (per-shell one-liners). No new document: completion is part of
installing/using the existing CLI, not a standalone concept (per docs/DOCS-GATE.md).
