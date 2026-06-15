---
# nixform2-8umq
title: E2.4 Provider registry resolve/download/verify/cache (NETWORK-GATED)
status: todo
type: feature
priority: normal
tags:
    - discovered
    - network-gated
created_at: 2026-06-15T11:12:17Z
updated_at: 2026-06-15T11:12:17Z
parent: nixform2-dwqg
---

ROADMAP 2.4 / CLAUDE.md §6: the OpenTofu provider registry is NOT reachable in this environment, so real-provider download is out of PoC scope and tracked here separately. When done in a network-enabled env, resolve a provider address -> download -> verify checksum/signature -> cache; then E2 codegen runs against the real schema. DESIGNATED FIRST REAL PROVIDERS: AWS (terraform-provider-aws — deep nested schemas, sets, sensitive values; stresses the type mapping) and Hetzner (terraform-provider-hcloud — small, clean; ideal for a real round trip, e.g. a server's computed IP feeding a DNS record). Do NOT make this a blocking step in the PoC; E2 is validated hermetically against the fake providers' schemas.
