---
# nixform2-oycy
title: 'A4: Shell completion'
status: todo
type: epic
priority: low
tags:
    - roadmap
created_at: 2026-06-16T13:37:59Z
updated_at: 2026-06-16T13:37:59Z
parent: nixform2-zdj0
---

Tab-completion for the CLI.

ROADMAP Phase A4. The user asked for autocomplete. GROUND TRUTH: cobra can generate bash/zsh/fish completion; nothing is wired today (no `completion` command, no ValidArgs).

## Scope
- `nivis completion <bash|zsh|fish>` via cobra's built-in generator.
- Dynamic completion of resource ids for `state show` and `--target`, read from the state file.
- Document install in INSTALL.md.
