---
# nixform2-n2rg
title: 'A5: Per-provider reference docs'
status: todo
type: epic
priority: normal
tags:
    - roadmap
created_at: 2026-06-16T13:37:59Z
updated_at: 2026-06-16T13:37:59Z
parent: nixform2-zdj0
---

Stop making users mentally translate HCL to Nivis: give them provider argument reference in Nivis terms.

ROADMAP Phase A5. The user asked for "adapted documentation (terraform docs to Nivis)". GROUND TRUTH: schema codegen (nivis gen, Epic 2) already produces typed constructors from GetProviderSchema, so the schema is in hand.

## Scope
- Generate or curate a "Terraform docs -> Nivis" argument reference (e.g. aws_instance's args, discoverable in Nivis form).
- Couple to the existing codegen output where possible.
- Decide: generated doc pages vs a lookup, and where they live (docs site).
