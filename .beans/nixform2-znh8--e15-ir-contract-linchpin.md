---
# nixform2-znh8
title: E1.5 IR contract (linchpin)
status: completed
type: epic
priority: high
tags:
    - critical-path
created_at: 2026-06-15T09:02:40Z
updated_at: 2026-06-15T09:18:49Z
parent: nixform2-hj4w
blocked_by:
    - nixform2-enm7
---

Author & freeze docs/IR-CONTRACT.md. WRITE FIRST. Worked OpenSpec change exists at openspec/changes/define-ir-contract/ (validated). Blocks E2/E3/E3.5. OpenSpec changes: define-ir-contract.



## Summary of Changes
OpenSpec change `define-ir-contract` implemented and archived as
`2026-06-15-define-ir-contract`. The IR contract is now frozen AND machine-checkable:

- `docs/IR-CONTRACT.md` — authoritative prose; Validation section now points at the
  schema + conformance suite and states the toIR / IngestIR obligations.
- `docs/ir-schema.json` — normative JSON Schema (Draft 2020-12) for the IR shape and
  the __ref/__derived/__sensitiveRef leaf encodings; marker objects dispatched via
  if/then so malformed leaves report an addressed error (names the offending path).
- `tests/ir-conformance/` — executable checker (structural JSON-Schema + referential
  rules JSON Schema can't express: unique ids, declared providers, edge endpoints,
  ref targets) with valid/ + invalid/ fixtures. Suite passes 7/7; each invalid
  fixture asserts the error names the offending element.
- `openspec/specs/ir/spec.md` — source-of-truth spec populated with 7 requirements.

Tests: `python3 tests/ir-conformance/check.py test` -> 7/7. `openspec validate ir` passes.
This unblocks E4a (fake providers target the frozen IR), E3 (executor ingests it),
and E3.5 (phased loop).
