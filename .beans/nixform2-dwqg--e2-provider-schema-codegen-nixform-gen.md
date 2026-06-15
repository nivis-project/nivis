---
# nixform2-dwqg
title: E2 Provider schema codegen (nixform-gen)
status: completed
type: epic
priority: normal
tags:
    - off-critical-path
created_at: 2026-06-15T09:02:41Z
updated_at: 2026-06-15T11:18:53Z
parent: nixform2-hj4w
blocked_by:
    - nixform2-qv4t
---

OFF critical path; build AFTER E4b. Schema->Nix type model + constructor codegen + override seam. Registry download is network-gated, separate bean. OpenSpec changes: (record here).



OpenSpec changes: schema-codegen. Registry download network-gated (beans-8umq; first real targets AWS + Hetzner).



## Summary of Changes
OpenSpec change schema-codegen (archived). Generic schema->Nix codegen, validated
hermetically against the fake providers (per the AWS/Hetzner scoping decision —
those are recorded as the network-gated first real targets, beans-8umq):
- internal/gen: schema fetch (spawn + GetProviderSchema), type map (scalars,
  list/set/map, nested object, roles incl. sensitive; computed-only -> output),
  emit (constructor with friendly required-throw, optional passthrough, override
  seam, deterministic).
- cmd/nixform-gen: --provider/--identity/--out CLI.
- Tests: synthetic-schema type-map units + emit structure + e2e generating
  against the real provider-alpha and evaluating the result.
Deferred (network-gated): registry download w/ AWS+Hetzner first (beans-8umq);
flake-app packaging (beans-28sn). E2 core complete.
