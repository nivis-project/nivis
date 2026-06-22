---
# nixform2-izhk
title: 'B1: Remote state backend (S3 first)'
status: in-progress
type: epic
priority: normal
tags:
    - roadmap
created_at: 2026-06-16T13:38:17Z
updated_at: 2026-06-22T14:11:34Z
parent: nixform2-kovh
---

Implement the Store seam against S3. The user's #1 personal priority: "remote state using s3".

GROUND TRUTH: internal/state has a Store interface; only local JSON is implemented.

## Scope
- S3 backend: object per state, server-side encryption, the AWS credential chain Nivis already uses.
- Format stays Nivis's own (NO tfstate compatibility, DESIGN).
- Configured in the flake, not via env soup.
- Reuses the A6 state pull/push shape.



## OpenSpec changes
- ir-backend-block (archived 2026-06-22-ir-backend-block): the optional IR `backend` field + toIR/toModuleIR pass-through + Go ingest/validation (static, type required, no refs). Contract groundwork.
- s3-state-backend (next): the S3 Store impl (AWS SDK + endpoint), in-repo fake S3, CLI backend selection, hermetic e2e.
