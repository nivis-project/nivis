---
# nixform2-1okn
title: Enterprise-credible
status: todo
type: milestone
priority: low
tags:
    - roadmap
created_at: 2026-06-16T13:38:35Z
updated_at: 2026-06-16T13:38:35Z
---

ROADMAP Phase C (the longer horizon, after Phases A and B). See docs/ROADMAP.md. NixOS is gaining enterprise traction; this is where Nivis earns a seat there. Deliberately later; several epics are large enough to become their own milestones.

## Epics
- C1 Policy as code / guardrails
- C2 RBAC, teams, audit
- C3 Provider registry integration (network-gated, CLAUDE.md S6)
- C4 Secrets at scale
- C5 Scale and performance

Invariants per DESIGN.md: optimise only measured problems (no premature live-evaluator); registry work is network-gated.
