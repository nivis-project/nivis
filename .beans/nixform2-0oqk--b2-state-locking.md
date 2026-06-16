---
# nixform2-0oqk
title: 'B2: State locking'
status: todo
type: epic
priority: normal
tags:
    - roadmap
created_at: 2026-06-16T13:38:17Z
updated_at: 2026-06-16T13:38:17Z
parent: nixform2-kovh
---

A lock so two concurrent applies cannot corrupt shared state.

## Scope
- DynamoDB-style advisory lock for the S3 backend.
- `force-unlock` escape hatch.
- Clear "who holds the lock / since when" errors.
Depends on B1.
