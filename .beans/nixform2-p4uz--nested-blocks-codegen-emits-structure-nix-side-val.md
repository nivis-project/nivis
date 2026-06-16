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
updated_at: 2026-06-17T23:33:58Z
parent: nixform2-zdj0
---

The second half of nixform2-krwc, deferred from the nested-block-errors change (archived 2026-06-17-nested-block-errors, which fixed the executor error only).

Two remaining gaps so the nested-block list-vs-single mistake is prevented, not just explained:
1. CODEGEN: `nivis gen` emits only FLAT attributes; nested blocks aren't in the generated constructor, so users hand-write them and guess list-vs-single. Emit nested-block structure WITH correct list/single nesting in the generated constructors (the schema has block nesting modes: single/list/set/map).
2. NIX-SIDE VALIDATION: validate config against the provider schema at eval time, so the mistake is caught before apply rather than against a real provider.

Both are larger than the error-message fix and overlap codegen / A5. The actionable executor error (done) is the immediate mitigation; this makes it impossible to get wrong. See openspec/specs/executor (Requirement: Actionable error for nested-block list-vs-single mistakes).
