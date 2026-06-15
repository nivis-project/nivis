---
# nixform2-xtgz
title: 'E3b: refresh + destroy engines and CLI'
status: completed
type: epic
priority: normal
tags:
    - off-critical-path
created_at: 2026-06-15T10:46:01Z
updated_at: 2026-06-15T10:58:44Z
parent: nixform2-hj4w
---

ROADMAP Epic 3b (off critical path). Destroy engine: reverse-dependency-order ApplyResourceChange with null planned-state; honor preventDestroy. Refresh: ReadResource reconcile without changing the plan. CLI: plan/apply/destroy/refresh/state {list,show,rm}, --target. The fake providers already implement the destroy (null new-state) and read (passthrough) RPCs, so this is executor-side work. Completes the destroy/refresh assertions in docs/TESTING.md's headline e2e that E4b deferred.



OpenSpec changes: refresh-destroy-cli.



## Summary of Changes
OpenSpec change refresh-destroy-cli (archived 2026-06-15-refresh-destroy-cli).
- internal/graph: DestroyOrder (reverse dep order); Build now also derives
  ordering deps from __derived inputs (Nix-mediated deps matter for ordering).
- internal/destroy: reverse-order ApplyResourceChange(null) + state delete,
  honors preventDestroy, --target.
- internal/refresh: ReadResource reconcile, no plan/apply.
- internal/tfvalue: EncodeState + NullState.
- cmd/nixform: cobra CLI plan/apply/destroy/refresh + state list/show/rm,
  --flake/--state/--attr/--target. Smoke-tested end-to-end against the fakes.
- e2e lifecycle test: refresh leaves converged state unchanged; destroy removes
  C,B,A in reverse order. This satisfies the destroy/refresh assertions deferred
  from E4b, so docs/TESTING.md's headline e2e is now FULLY satisfied.
