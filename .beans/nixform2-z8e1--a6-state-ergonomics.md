---
# nixform2-z8e1
title: 'A6: State ergonomics'
status: todo
type: epic
priority: normal
tags:
    - roadmap
created_at: 2026-06-16T13:37:59Z
updated_at: 2026-06-16T13:37:59Z
parent: nixform2-zdj0
---

The small state-handling things a real project needs day to day, and the seam Phase B's remote backend will reuse.

ROADMAP Phase A6. GROUND TRUTH: configurable --state path and `state list/show/rm` exist; the Store interface is the remote-backend seam.

## Scope
- Polish `state list/show/rm`.
- A `state pull/push` shape that the Phase B remote backend (B1) will reuse.
- Clear errors on stale or locked state files.
