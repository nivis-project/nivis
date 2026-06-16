---
# nixform2-kym5
title: 'A1: Variables and overrides'
status: todo
type: epic
priority: high
tags:
    - roadmap
created_at: 2026-06-16T13:37:59Z
updated_at: 2026-06-16T13:37:59Z
parent: nixform2-zdj0
---

First-class inputs to a plan, so config is parameterised instead of hard-coded.

ROADMAP Phase A1. GROUND TRUTH: today there is no --var flag; ledger.vars is just whatever the flake author sets (e.g. the EC2 example reads ledger.vars.suffix). No CLI var injection or precedence exists.

## Scope
- Typed variables with defaults in the Nix config.
- CLI injection: `--var name=value` and `--var-file <file>`.
- Precedence: defaults < var-file < --var flag < environment (decide and spec).
- Threads through the phased-eval loop cleanly; the ledger already carries `vars`, formalise it.
- Stays pure: no impurity leaks into nix eval.

## Likely an IR-contract touch
How vars enter the `nivis.plan` function may need an IR-CONTRACT.md addition. If so, OpenSpec change to the contract FIRST (hard gate).

Tasks become OpenSpec changes. Tested against in-repo fakes.
