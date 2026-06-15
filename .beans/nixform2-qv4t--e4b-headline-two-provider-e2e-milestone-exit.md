---
# nixform2-qv4t
title: E4b Headline two-provider e2e (milestone exit)
status: completed
type: epic
priority: high
tags:
    - critical-path
created_at: 2026-06-15T09:02:41Z
updated_at: 2026-06-15T10:49:47Z
parent: nixform2-hj4w
blocked_by:
    - nixform2-aoss
---

Two providers, unknowns both sides, >=3 phases, Nix consumer reads both. Full spec docs/TESTING.md. This passing == milestone core done. OpenSpec changes: headline-e2e. destroy/refresh deferred to E3b (beans-xtgz).



## Summary of Changes
MILESTONE CORE ACHIEVED. OpenSpec change headline-e2e (archived 2026-06-15-headline-e2e).
tests/e2e/headline_test.go drives the REAL flake + REAL fake binaries through the
phase driver:
- TestHeadlineRoundTrip: two providers, unknowns both sides, exactly 3 apply
  phases to fixpoint; ledger has A.id/A.value/B.endpoint/C.*; systemConfig
  consumer concrete from BOTH providers (recordEndpoint=beta://rec-alpha::0,
  tokenValue=alpha::0, combined=beta://rec-alpha::0::alpha::0).
- TestTwoPhaseCapInsufficient: 2-phase cap fails (N>2 required, not incidental).
- TestCycleRejected: cyclic nixform.planCycle rejected at fixpoint naming A and C.

Deferred per scoping decision: destroy (reverse-order) + refresh (ReadResource)
assertions -> E3b (beans-xtgz, off critical path). All other docs/TESTING.md
headline assertions pass. The round-trip thesis is validated and trustworthy.
