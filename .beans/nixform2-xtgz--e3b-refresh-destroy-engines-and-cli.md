---
# nixform2-xtgz
title: 'E3b: refresh + destroy engines and CLI'
status: todo
type: epic
priority: normal
tags:
    - off-critical-path
created_at: 2026-06-15T10:46:01Z
updated_at: 2026-06-15T10:46:01Z
parent: nixform2-hj4w
---

ROADMAP Epic 3b (off critical path). Destroy engine: reverse-dependency-order ApplyResourceChange with null planned-state; honor preventDestroy. Refresh: ReadResource reconcile without changing the plan. CLI: plan/apply/destroy/refresh/state {list,show,rm}, --target. The fake providers already implement the destroy (null new-state) and read (passthrough) RPCs, so this is executor-side work. Completes the destroy/refresh assertions in docs/TESTING.md's headline e2e that E4b deferred.
