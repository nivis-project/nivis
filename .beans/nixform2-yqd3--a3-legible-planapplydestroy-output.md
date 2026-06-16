---
# nixform2-yqd3
title: 'A3: Legible plan/apply/destroy output'
status: todo
type: epic
priority: normal
tags:
    - roadmap
created_at: 2026-06-16T13:37:59Z
updated_at: 2026-06-16T13:37:59Z
parent: nixform2-zdj0
---

Make the plan readable: colorise by change type and make the phased nature visible. Pure DX, no behaviour change.

ROADMAP Phase A3. The user explicitly asked for colored output of plan/apply/destroy. GROUND TRUTH: a branded splash exists (cmd/nivis/splash.go) but the plan/apply diff is not colorised by change type.

## Scope
- Colorise: + create (green), ~ update (yellow), -/+ replace (red+green), - destroy (red), = no-op (dim).
- Summarise counts; show which resources resolved in which phase.
- Respect NO_COLOR and non-TTY (no escape codes when piped).
