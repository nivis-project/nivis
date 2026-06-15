---
# nixform2-enm7
title: E1 Nix library core (nixform-lib)
status: completed
type: epic
priority: high
tags:
    - critical-path
created_at: 2026-06-15T09:02:40Z
updated_at: 2026-06-15T11:08:11Z
parent: nixform2-hj4w
---

mkResource, reference system, meta-args, module system, IR serializer, flake interface. Tasks tracked as OpenSpec changes. See ROADMAP.md Epic 1. OpenSpec changes: nix-lib-core (mkResource/refs/derived/toIR/plan); module-system + for_each to follow.



## Progress
- [x] nix-lib-core (archived 2026-06-15-nix-lib-core): mkResource, reference
  system (__ref/__derived via refAttr + str/derived), toIR (contract-conforming,
  ledger-resolving, edge derivation), flake nixform.plan interface, self-contained
  minilib (no nixpkgs). Nix property tests + conformance pipe + phased-resolution
  test all green. Proven: phase-by-phase ledger resolution reproduces the headline
  3-phase progression on the Nix side; IR validates against ir-schema.json.
- [ ] module-system (lib.evalModules, {config,tf,pkgs,lib,...}) + for_each/count
  expansion: deferred follow-up change. NOT on the round-trip critical path.
E1 stays in-progress until those land; E3.5 is unblocked by nix-lib-core.



OpenSpec changes (E1): nix-lib-core (done), nix-lib-modules (this, completes E1).



## Summary of Changes
E1 COMPLETE via two OpenSpec changes:
- nix-lib-core (2026-06-15-nix-lib-core): mkResource, refs (__ref/__derived),
  toIR, flake plan interface, self-contained minilib.
- nix-lib-modules (2026-06-15-nix-lib-modules): count/for_each expansion in Nix
  (mkResources, ids <base>__<index|key>); module system (evalModules/toModuleIR)
  merging resources across modules into one flat graph with cross-module tf refs
  and duplicate-id rejection. 7 module/expansion property tests + conformance
  green. The nix-lib capability spec now has 8 requirements.
