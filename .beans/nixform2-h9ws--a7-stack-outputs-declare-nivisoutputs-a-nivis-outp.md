---
# nixform2-h9ws
title: 'A7: Stack outputs (declare nivis.outputs + a nivis output command)'
status: todo
type: epic
priority: normal
tags:
    - roadmap
    - discovered
created_at: 2026-06-18T06:00:00Z
updated_at: 2026-06-18T06:00:00Z
parent: nixform2-zdj0
---

Surface named values OUT of a run, the Terraform `output "x" {}` equivalent. Discovered while reviewing datasources (A2): the user asked whether outputs are needed to complete datasources.

## Why (and the relationship to datasources)
Datasources are COMPLETE without this: a datasource's attributes already flow back IN via mkData refAttr -> the ledger -> resources / Nix (the round trip). This epic is the orthogonal OTHER meaning of "outputs": named values surfaced OUT of the whole run for a human to read or for a downstream consumer. It applies equally to resource and datasource attributes, so it is not datasource-specific, but datasources make the gap more visible.

## Ground truth (what exists today)
- The flake interface is just `nivis.plan` (E1.6 originally envisioned `nivis.state`/`nivis.providers` outputs too; not shipped).
- The IR already has `nixConsumers` (values Nix computed FROM resource/datasource outputs, surfaced for the round trip) and the *->Nix classification. That is the INTERNAL plumbing this feature builds on; what's missing is a USER-FACING "these are my stack's outputs" surface + a way to read them.
- Today a user inspects values via `nivis state show <id>`. There is no `nivis output`.

## Scope (to spec via OpenSpec; spec-before-code)
- A Nix way to DECLARE named outputs (e.g. `nivis.outputs = { ip = instance.refAttr "public_ip"; }` or an `outputs` arg to toIR), likely reusing/renaming the nixConsumers plumbing so a declared output is a first-class consumer with a stable name.
- `nivis output [name]`: print resolved outputs after apply (all, or one), human-readable; respect NO_COLOR/non-TTY (pairs with A3).
- A MACHINE-READABLE published form (e.g. `nivis output --json`, or a written outputs.json) so another tool / CI step / another Nivis stack can consume this stack's results. Decide the artifact + path in the spec.
- Sensitive outputs follow the existing 0600/sensitive-ref handling (never world-readable).
- IR-CONTRACT touch likely (formalising a named-output node on top of nixConsumers) -> OpenSpec change to the contract FIRST (hard gate).

## Non-goals (for the first cut)
- Remote/shared output state (that pairs with B1 remote state).
- Cross-stack auto-wiring / a registry of stacks' outputs (later).

Tested against in-repo fakes (a declared output resolves across phases and prints/serialises). Docs-coverage gate: likely a paragraph in an existing doc or a short OUTPUTS section (decide per docs/DOCS-GATE.md at implementation time; may not need a whole new document).
