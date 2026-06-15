---
# nixform2-f1yp
title: beans roadmap renders empty for todo milestones
status: todo
type: bug
priority: low
tags:
    - discovered
created_at: 2026-06-15T09:03:21Z
updated_at: 2026-06-15T09:03:21Z
parent: nixform2-hj4w
---

Discovered while bootstrapping (beans 1.4.1). `beans roadmap` and `beans roadmap --json` return no milestones even though a milestone with epic children exists in `beans list` (status=todo). Not blocking: `beans list` and `beans list --type epic --ready` work and are what the autonomous loop relies on. Investigate roadmap's milestone-status default/filter; cosmetic only.
