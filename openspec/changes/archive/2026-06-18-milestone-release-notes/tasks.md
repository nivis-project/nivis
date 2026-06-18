# Tasks: milestone-release-notes

## 1. Tutorial highlight markers
- [x] 1.1 In `docs/TUTORIAL-FEATURES.md`, wrap a few key blocks in
      `<!-- release-note: <title> -->` ... `<!-- /release-note -->` (variables,
      the phased apply, outputs). Each block is verified text already in the doc.

## 2. Generator
- [x] 2.1 `scripts/milestone-notes.sh <milestone-id>`: emit
      `docs/releases/<slug>.md` with a header (milestone title + DoD), Highlights
      (the marked tutorial blocks, verbatim), What shipped (completed child epic
      titles from `beans`), and the CHANGELOG `[Unreleased]` section.
      Deterministic: stable ordering, no per-run timestamps. Pure read + write.
- [x] 2.2 `<slug>` derives from the milestone title (kebab-case) or the id; pick
      one and keep it stable.

## 3. Gate
- [x] 3.1 `tests/check-milestone-notes.sh`: for every COMPLETED milestone (from
      `beans`), assert `docs/releases/<slug>.md` exists and regenerates identically
      (run the generator to a temp file, diff). Fail with the milestone name and a
      pointer to the generator. Skip cleanly if `beans` is unavailable.
- [x] 3.2 Chain it into `tests/check-docs-ssot.sh`.

## 4. First artifact + docs
- [x] 4.1 Generate `docs/releases/road-to-v1.md` (the milestone is feature-complete;
      this is the first real notes doc). Commit it.
- [x] 4.2 `docs/RELEASING.md`: a "Closing a milestone" section (mark the milestone
      completed in beans, run `scripts/milestone-notes.sh`, the gate keeps it
      current). No em dashes.

## 5. Gate run
- [x] 5.1 `bash tests/check-milestone-notes.sh` green; `bash tests/check-docs-ssot.sh` green.
- [x] 5.2 `openspec validate milestone-release-notes --strict`; archive; close beans-tvin.
