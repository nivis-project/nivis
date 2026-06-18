---
# nixform2-hj4w
title: 'M0: nixform PoC / alpha base'
status: completed
type: milestone
priority: high
tags:
    - poc
created_at: 2026-06-15T09:02:40Z
updated_at: 2026-06-18T15:54:44Z
---

Exit criterion: headline two-provider e2e passes (unknowns on both sides, >=3 phases, Nix-side consumer reads both providers). See ROADMAP.md and docs/TESTING.md. cc administers this milestone and its epics.



## Summary
MILESTONE COMPLETE. All 9 epics done. The headline two-provider round trip passes
end to end (tests/e2e: 3 phases to fixpoint, unknowns on both sides, both-providers
consumer concrete, cycle rejected, destroy/refresh in order). Built: Nix library
(mkResource/refs/toIR/modules/expansion), frozen IR contract + machine-checkable
schema, Go executor (ingest/DAG/plugin-client/plan/apply/phased-loop/destroy/
refresh), fake tfprotov6 providers, schema codegen, CLI, error UX + docs.
Network-gated follow-ups tracked: registry download w/ AWS+Hetzner (beans-8umq),
flake-app packaging (beans-28sn).
