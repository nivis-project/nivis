---
# nixform2-tyzs
title: 'B3: Drift detection'
status: todo
type: epic
priority: normal
tags:
    - roadmap
created_at: 2026-06-16T13:38:17Z
updated_at: 2026-06-16T13:38:17Z
parent: nixform2-kovh
---

A real "plan shows drift" experience.

GROUND TRUTH: `refresh` (ReadResource reconcile) exists; surfacing drift in plan is the gap.

## Scope
- Reconcile remote reality vs stored state; surface out-of-band changes in the plan.
- Drift-injecting fake provider for hermetic tests.
