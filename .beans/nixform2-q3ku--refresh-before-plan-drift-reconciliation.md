---
# nixform2-q3ku
title: Refresh-before-plan / drift reconciliation
status: todo
type: feature
priority: normal
tags:
    - discovered
created_at: 2026-06-15T17:50:08Z
updated_at: 2026-06-15T17:50:08Z
parent: nixform2-ft9v
---

resource-update-replace plans against STORED state, not a fresh provider read. If the real resource drifted out-of-band, the plan won't see it. Add an opt (or default) to ReadResource the prior state before planning (the refresh path already exists) so the plan reflects reality, and reconcile refresh with the plan/apply loop. Non-goal of resource-update-replace; future milestone.
