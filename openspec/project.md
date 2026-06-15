# OpenSpec — project conventions (terrae nivis)

This project uses OpenSpec for spec-driven development. The spec is the source
of truth; code follows an approved change.

## Layout
- `openspec/specs/` — current source-of-truth specifications (what IS).
- `openspec/changes/<change-id>/` — proposed changes (what SHOULD change):
  - `proposal.md` — why + what (the summary and motivation).
  - `tasks.md` — implementation checklist (the ordered work).
  - `specs/<capability>/spec.md` — spec **deltas**: requirements marked
    `ADDED` / `MODIFIED` / `REMOVED`, each with `GIVEN / WHEN / THEN` scenarios.
  - `design.md` — optional, for non-trivial technical approach.

## Workflow (per change)
1. `openspec` propose → scaffold the change folder.
2. Write/refine `proposal.md`, `tasks.md`, and the spec deltas.
3. `openspec validate <change-id>` must pass. **Do not implement before this.**
4. Implement `tasks.md` in order; each task gets a test (see docs/TESTING.md).
5. `openspec archive <change-id>` when complete; fold deltas into `specs/`.

## Relationship to beans
- A beans **epic** maps to one or more OpenSpec **changes**.
- Record the change-id(s) in the epic's beans body so they stay linked.
- Closing work references the bean in the commit: `... Closes beans-XXXX.`

## The IR contract is special
`docs/IR-CONTRACT.md` is the frozen cross-epic API. Any change to its shape
starts with an OpenSpec change against it (see `changes/define-ir-contract/`),
then downstream changes update producers/consumers. Never edit the IR shape
ad hoc.

## Conventions
- Each change is small and independently reviewable/archivable.
- Spec deltas describe **behavior** (GIVEN/WHEN/THEN), not implementation.
- Non-goals are explicit in every `proposal.md`.
