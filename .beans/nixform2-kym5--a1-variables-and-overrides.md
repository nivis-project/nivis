---
# nixform2-kym5
title: 'A1: Variables and overrides'
status: completed
type: epic
priority: high
tags:
    - roadmap
created_at: 2026-06-16T13:37:59Z
updated_at: 2026-06-17T23:53:30Z
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


---
## Summary of Changes
DONE via OpenSpec change variables-and-overrides (archived 2026-06-17-variables-and-overrides). First-class config variables:

- NIX: nivis.mkVars { name = { type ? "any"; default ? ...; }; } (ledger.vars or {}) resolves declared vars against the injected ledger.vars: set value wins (type-checked str/int/bool/any), unset falls back to default, unset+no-default is REQUIRED (throws naming the var), wrong type throws naming var+type, undeclared injected values ignored. Pure (builtins only, no IO). Exported from nix/lib/default.nix.
- EXECUTOR: --var name=value and --var-file <json> (both repeatable) + NIVIS_VAR_<name> env. Terraform precedence (lowest->highest): defaults(Nix) < env < var-file(later wins) < --var(later wins), so an explicit flag always wins (avoids the stale-env footgun; eases TF migration). internal/vars resolves once; values travel only in the 0600 ledger file (purity/secret handling preserved).
- IR: the outputs ledger gains an optional `vars` object, constant across all phases (variables are known inputs, never refs/unknowns). IR-CONTRACT.md + ir spec updated (the frozen-contract gate). ir-schema.json untouched (it describes IR output, not the ledger input); conformance stays green.
- DECISION (with maintainer): mkVars now, NOT module-system options. Resolved values land in ledger.vars, the same seam a future options.vars module layer could write to, so the powerful module approach stays an additive future with no rework (deferred). Precedence = Terraform's.

TESTS: Go internal/vars (precedence, later-wins, malformed --var, unreadable/non-object --var-file, typed JSON values, empty->nil); internal/ledger (vars marshaled/omitted); internal/phase integration (a var flows ledger->eval->config->apply via the real fake provider; vars constant across phases). Nix property P9 (the five mkVars scenarios). Full gate green: gofmt, go build, go test, run-nix-tests (P9 + IR conformance), check-docs-ssot (mdbook build).

DOCS: AWS S3 tutorial gained a "parameterise with a variable" subsection; README Nix-library list gained mkVars. No em dashes.

NON-GOALS (deferred): module-system declaration; rich types beyond str/int/bool/any; .auto.tfvars auto-loading; sensitive variables.
