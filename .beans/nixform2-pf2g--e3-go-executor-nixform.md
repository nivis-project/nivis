---
# nixform2-pf2g
title: E3 Go executor (nixform)
status: completed
type: epic
priority: high
tags:
    - critical-path
created_at: 2026-06-15T09:02:41Z
updated_at: 2026-06-15T10:07:54Z
parent: nixform2-hj4w
blocked_by:
    - nixform2-znh8
    - nixform2-kxlp
---

IR ingestion, trivial JSON state, plugin manager (spawn, plain v6 handshake), DAG + TF->TF resolution, plan/apply, refresh/destroy/CLI. ROADMAP Epic 3. OpenSpec changes: executor-core (this), then plugin-client-plan-apply (beans-uu26).



## Progress
- [x] executor-core (archived 2026-06-15-executor-core): IR ingest/validate,
  ref classification (TF->TF vs *->Nix), DAG + cycle detection + ready-set,
  TF->TF resolution (nested paths), lockable JSON state. internal/ir,
  internal/graph, internal/state. Tests green.
- [ ] plugin-client-plan-apply (beans-uu26): spawn provider, go-plugin v6
  handshake via generated tfplugin6 client stubs, plan/apply engines,
  __ref->tfprotov6-unknown mapping. NOT yet started.
E3 stays in-progress until plugin-client-plan-apply lands.



## Summary of Changes
E3 complete via two OpenSpec changes:
- executor-core (2026-06-15-executor-core): IR ingest/validate, ref
  classification, DAG + cycle detection + ready-set, TF->TF resolution, lockable
  JSON state. internal/ir, internal/graph, internal/state.
- plugin-client-plan-apply (2026-06-15-plugin-client-plan-apply): generated
  tfplugin6 client, go-plugin v6 plugin manager (spawn + handshake + pooling),
  value codec (__ref->unknown), plan + apply engines. Integration test drives
  the real fake providers end-to-end.
The executor capability spec now has 10 requirements. Single-provider round trip
proven (DESIGN D5). Unblocks E3.5 (phased evaluation to fixpoint).
