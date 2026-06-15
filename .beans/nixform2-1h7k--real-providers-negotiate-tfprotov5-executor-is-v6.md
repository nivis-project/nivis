---
# nixform2-1h7k
title: Real providers negotiate tfprotov5; executor is v6-only
status: completed
type: bug
priority: high
tags:
    - discovered
created_at: 2026-06-15T12:44:15Z
updated_at: 2026-06-15T13:15:18Z
parent: nixform2-8umq
---

Discovered starting nixform2-8umq (2026-06-15). The OpenTofu registry + GitHub releases ARE now reachable from this env (contrary to the original CLAUDE.md §6 assumption), so provider download/verify/cache is buildable. BUT: the two intended first providers advertise only protocol 5.0:
  - hashicorp/aws 6.50.0  -> protocols ['5.0']
  - hetznercloud/hcloud 1.65.0 -> protocols ['5.0']
The executor + generated stubs speak tfprotov6 ONLY (internal/tfplugin6,
internal/plugin handshake ProtocolVersion=6). A v5 provider will fail the v6
handshake. Most real providers still negotiate v5.

Options:
  (A) Add a tfprotov5 client path (vendor tfplugin5.proto, generate stubs, a v5
      plugin handshake + value codec) and either run v5 directly or mux v5->v6.
      Largest effort; DESIGN parked 'no muxer for the PoC'.
  (B) Demo against a v6-only real provider (some providers offer protocol 6) to
      prove the registry->download->spawn->schema path with real bits, deferring
      broad v5 support.
  (C) Keep nixform2-8umq scoped to download/verify/cache + schema fetch (which is
      protocol-version agnostic up to GetProviderSchema? — verify) and treat
      live plan/apply against v5 as separate.
Decision needed before building.



## Resolved
tfprotov5 support added (OpenSpec tfprotov5-client): generated v5 stubs, v5
backend behind provider.Client, manager negotiates v5/v6 by advertised protocol.
Real v5 providers (hcloud) now spawn and serve schema.
