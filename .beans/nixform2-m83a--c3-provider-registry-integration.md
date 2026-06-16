---
# nixform2-m83a
title: 'C3: Provider registry integration'
status: todo
type: epic
priority: low
tags:
    - roadmap
created_at: 2026-06-16T13:38:35Z
updated_at: 2026-06-16T13:38:35Z
parent: nixform2-1okn
---

Real provider download/verify/cache from the OpenTofu registry, hardened for enterprise.

NETWORK-GATED per CLAUDE.md S6 (the registry is not reachable in the dev sandbox; use fakes there). GROUND TRUTH: providers are fetched on first use today, but this needs hardening.

## Scope
- Offline / air-gapped mirrors.
- Supply-chain verification (checksums, signatures).
