---
# nixform2-q3ku
title: Refresh-before-plan / drift reconciliation
status: completed
type: feature
priority: normal
tags:
    - discovered
created_at: 2026-06-15T17:50:08Z
updated_at: 2026-06-15T20:38:32Z
parent: nixform2-ft9v
---

resource-update-replace plans against STORED state, not a fresh provider read. If the real resource drifted out-of-band, the plan won't see it. Add an opt (or default) to ReadResource the prior state before planning (the refresh path already exists) so the plan reflects reality, and reconcile refresh with the plan/apply loop. Non-goal of resource-update-replace; future milestone.


---
DONE via OpenSpec change refresh-before-plan (archived 2026-06-15-refresh-before-plan). plan/apply now refresh prior state before planning by default: Driver.priorState reads each in-state resource through the provider (drift -> plan against the real state; empty read = deleted out-of-band -> plan/apply as a create); --refresh=false (Driver.NoRefresh) opts out and uses stored state. Wired into PlanReport + applyOne; --refresh flag on plan/apply (default true). Unit tests: drift uses read state, deletion re-creates (no destroy, nil prior), --refresh=false skips Read. Full gate green (build, go test incl. the fake-provider round trips that now refresh, nix, IR conformance 7/7). The refresh command primitive (internal/refresh) was the basis; this reuses provider.Client.Read.
