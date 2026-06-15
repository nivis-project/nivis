---
# nixform2-2vc3
title: 'M2: Real provider support (tfprotov5 + registry)'
status: in-progress
type: epic
priority: high
tags:
    - milestone-2
created_at: 2026-06-15T12:49:06Z
updated_at: 2026-06-15T12:50:04Z
parent: nixform2-hj4w
---

Second milestone: drive REAL providers (AWS, Hetzner) end to end. The registry + GitHub releases are reachable now (env changed since the PoC). Two blockers from the PoC: (1) real providers speak tfprotov5, executor is v6-only (beans-1h7k); (2) provider download/verify/cache (beans-8umq). OpenSpec changes:   1. provider-abstraction — version-neutral provider.Client interface; refactor v6      behind it (no behavior change).   2. tfprotov5-client — generate tfplugin5 stubs, a v5 backend, manager negotiates      v5/v6 by advertised protocols.   3. provider-registry — resolve -> download (GitHub) -> SHA256 verify -> cache. Then a real-provider e2e (Hetzner hcloud: a server's computed value feeding a dependent resource) proves the round trip against real bits. Closes beans-8umq and beans-1h7k. AWS + Hetzner are the first targets.



In progress: provider-abstraction (change 1/3).
