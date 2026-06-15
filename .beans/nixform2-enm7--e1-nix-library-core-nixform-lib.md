---
# nixform2-enm7
title: E1 Nix library core (nixform-lib)
status: in-progress
type: epic
priority: high
tags:
    - critical-path
created_at: 2026-06-15T09:02:40Z
updated_at: 2026-06-15T10:21:51Z
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
