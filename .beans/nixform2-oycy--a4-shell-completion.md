---
# nixform2-oycy
title: 'A4: Shell completion'
status: completed
type: epic
priority: low
tags:
    - roadmap
created_at: 2026-06-16T13:37:59Z
updated_at: 2026-06-18T08:00:00Z
parent: nixform2-zdj0
---

Tab-completion for the CLI.

ROADMAP Phase A4. The user asked for autocomplete. GROUND TRUTH: cobra can generate bash/zsh/fish completion; nothing is wired today (no `completion` command, no ValidArgs).

## Scope
- `nivis completion <bash|zsh|fish>` via cobra's built-in generator.
- Dynamic completion of resource ids for `state show` and `--target`, read from the state file.
- Document install in INSTALL.md.


---
## Summary of Changes
DONE via OpenSpec change shell-completion (archived 2026-06-18-shell-completion). Pure DX:

- `nivis completion <bash|zsh|fish|powershell>` via Cobra's built-in generator (auto-registered; kept, not disabled).
- DYNAMIC resource-id completion from the local state file: a stateIDs completer (cmd/nivis/completion.go) reads store.List() and returns the ids with ShellCompDirectiveNoFileComp (so a missing/empty store completes to nothing, never to filenames). Wired as ValidArgsFunction on `state show` and `state rm` (first arg only), and via RegisterFlagCompletionFunc for the --target persistent flag.

VERIFIED with the hidden __complete: `state show <TAB>` and `--target <TAB>` return the state ids (sorted, NoFileComp); empty/missing state returns nothing. Tests (cmd/nivis): populated store -> ids; missing store -> none + NoFileComp; second arg -> none. Full gate green: gofmt, go build, go test, check-docs-ssot (incl docs-coverage + mdbook).

DOCS (new section): docs/INSTALL.md "Shell completion" with the per-shell one-liners.

NON-GOALS (deferred): completing resource types / provider names / attribute names (needs schema or a flake eval at completion time); flag-value completion beyond --target.
