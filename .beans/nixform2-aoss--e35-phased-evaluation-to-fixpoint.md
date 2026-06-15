---
# nixform2-aoss
title: E3.5 Phased evaluation to fixpoint
status: completed
type: epic
priority: high
tags:
    - critical-path
created_at: 2026-06-15T09:02:41Z
updated_at: 2026-06-15T10:38:17Z
parent: nixform2-hj4w
blocked_by:
    - nixform2-pf2g
---

THE THESIS. Outputs ledger, phase driver, fixpoint/cycle detection, *->Nix feedback. Generalizes 2-phase to N-phase. ROADMAP Epic 3.5. OpenSpec changes: phased-eval. OpenSpec changes: (record here).



## Summary of Changes
THE THESIS, PROVEN. OpenSpec change phased-eval (archived 2026-06-15-phased-eval).
- internal/ledger: outputs ledger in the contract format; 0600 save (atomic),
  append/known/has, graph adapter.
- internal/phase: NixEvaluator seam (real `nix eval .#nixform.plan` + stub for
  hermetic tests); Driver.Run loop = eval -> ingest -> ResolveTFTF(ledger) ->
  apply now-ready resources via plan+apply -> append outputs -> repeat to
  fixpoint; stuck detection names unresolved resources + awaited inputs.
- Unit tests prove: 3-phase chain in exactly 3 apply phases; 2-phase cap FAILS
  (N>2 required, not incidental); stuck resource named; ledger 0600.
- Integration test (REAL nix eval + REAL fake binaries): the flake example
  resolves A->B->C across 3 phases to fixpoint; systemConfig consumer reading
  from BOTH providers is concrete and equals beta://rec-alpha::0::alpha::0.
This is the round trip the project exists to prove (DESIGN D3). Unblocks E4b.
