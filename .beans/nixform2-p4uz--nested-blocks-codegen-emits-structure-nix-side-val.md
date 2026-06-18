---
# nixform2-p4uz
title: 'Nested blocks: codegen emits structure + Nix-side validation'
status: todo
type: feature
priority: normal
tags:
    - discovered
    - roadmap
created_at: 2026-06-17T23:33:58Z
updated_at: 2026-06-18T09:33:03Z
parent: nixform2-zdj0
---

The second half of nixform2-krwc, deferred from the nested-block-errors change (archived 2026-06-17-nested-block-errors, which fixed the executor error only).

Two remaining gaps so the nested-block list-vs-single mistake is prevented, not just explained:
1. CODEGEN: `nivis gen` emits only FLAT attributes; nested blocks aren't in the generated constructor, so users hand-write them and guess list-vs-single. Emit nested-block structure WITH correct list/single nesting in the generated constructors (the schema has block nesting modes: single/list/set/map).
2. NIX-SIDE VALIDATION: validate config against the provider schema at eval time, so the mistake is caught before apply rather than against a real provider.

Both are larger than the error-message fix and overlap codegen / A5. The actionable executor error (done) is the immediate mitigation; this makes it impossible to get wrong. See openspec/specs/executor (Requirement: Actionable error for nested-block list-vs-single mistakes).


---
Doing GAP 1 (codegen emits nested-block structure) jointly with A5 (nixform2-n2rg) as OpenSpec change codegen-nested-blocks. DECISION (with maintainer): emit each nested block as a typed argument with the correct default shape ([] for list/set-nested, attrset for single) + a doc comment naming the nesting (NOT per-block builder helpers, which are heavier generator machinery). This prevents the list-vs-single trap (the krwc error half already explains it). GAP 2 (Nix-side schema validation at eval time) remains DEFERRED until there is demand.


---
GAP 1 DONE via codegen-nested-blocks (archived 2026-06-18-codegen-nested-blocks, jointly with A5): `nivis gen` now emits nested blocks as typed args with the correct per-nesting shape ([] list/set, null attrset single, {} map) + doc comments, so the list-vs-single mistake cannot be guessed. provider.ResourceSchema carries Blocks (recursive); v6/v5 populate from Block.BlockTypes; gen model + emit render them. Tested (gen emit, schema mapping, v6 backend surfacing) + live-eval verified.

REMAINING (this bean now tracks GAP 2 only): Nix-side / eval-time schema validation, so a shape mistake is caught at `nivis plan` before apply. DEFERRED until there is demand: the actionable executor error (krwc) already explains the mistake, and codegen (gap 1) now prevents the common case. Keeping this bean open as the tracker for gap 2; status back to todo.
