---
# nixform2-kovh
title: 'M2: Team-ready (shared state and locking)'
status: todo
type: milestone
priority: normal
tags:
    - roadmap
created_at: 2026-06-16T13:38:17Z
updated_at: 2026-06-18T15:54:44Z
---

ROADMAP Phase B (after Phase A, the Road-to-v1 daily-driver milestone). See docs/ROADMAP.md.

## Definition of done
Multiple people and CI can safely operate the same infrastructure concurrently.

## Epics
- B1 Remote state backend (S3 first)
- B2 State locking
- B3 Drift detection
- B4 Multiple environments

Invariants per DESIGN.md: state format is Nivis's own (NO tfstate compatibility guarantee); the Store interface is the seam.
