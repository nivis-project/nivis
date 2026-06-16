---
# nixform2-q7fx
title: 'C4: Secrets at scale'
status: todo
type: epic
priority: low
tags:
    - roadmap
created_at: 2026-06-16T13:38:35Z
updated_at: 2026-06-16T13:38:35Z
parent: nixform2-1okn
---

Secrets never transit the Nix store at all.

GROUND TRUTH: the IR already keeps sensitive values out of the world-readable store (0600 ledger, sensitive refs).

## Scope
- Integrate with real secret stores: Vault, AWS SSM, sops-nix.
- Extend the existing sensitive-value handling rather than replace it.
