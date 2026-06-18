---
# nixform2-n2rg
title: 'A5: Per-provider reference docs'
status: completed
type: epic
priority: normal
tags:
    - roadmap
created_at: 2026-06-16T13:37:59Z
updated_at: 2026-06-18T09:33:02Z
parent: nixform2-zdj0
---

Stop making users mentally translate HCL to Nivis: give them provider argument reference in Nivis terms.

ROADMAP Phase A5. The user asked for "adapted documentation (terraform docs to Nivis)". GROUND TRUTH: schema codegen (nivis gen, Epic 2) already produces typed constructors from GetProviderSchema, so the schema is in hand.

## Scope
- Generate or curate a "Terraform docs -> Nivis" argument reference (e.g. aws_instance's args, discoverable in Nivis form).
- Couple to the existing codegen output where possible.
- Decide: generated doc pages vs a lookup, and where they live (docs site).


---
Doing A5 jointly with p4uz-gap1 as one codegen-completion pass (OpenSpec change codegen-nested-blocks). DECISION (with maintainer): the generated constructor IS the per-provider argument reference; A5 is just a short doc paragraph pointing at `nivis gen` output. NO separate Markdown reference site / hand-maintained pages (too early; would rot). Linked: p4uz (nixform2-p4uz).


---
## Summary of Changes
DONE via OpenSpec change codegen-nested-blocks (archived 2026-06-18-codegen-nested-blocks), jointly with p4uz gap 1. DECISION (with maintainer): the generated constructor IS the per-provider argument reference; no separate Markdown doc site. `nivis gen` now emits complete constructors (every flat arg + every nested block with the correct list/single/set/map shape + computed outputs), each documented in the generated file. A5's deliverable is a docs note (docs/GETTING-STARTED.md s6 + README) telling users to run `nivis gen` and read the .nix as the reference. Verified live: a generated constructor with a list-nested + single-nested block evaluates correctly against the real lib.
