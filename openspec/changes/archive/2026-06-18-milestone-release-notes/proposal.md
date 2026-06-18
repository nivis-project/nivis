# Proposal: milestone-release-notes

## Why
A version's changelog says *what changed*; it does not show *what a user can now
do*. When a whole **milestone** closes (a coherent batch of capability, e.g.
"Road to v1"), the release notes should feature concrete, runnable examples, the
same verified ones from the tutorials, so a reader sees the capability, not a list
of internals. Today there is nothing that assembles this, and the tutorial
examples (which are tested and real) are not reused. This adds a milestone-closing
gate that generates release notes from the milestone's epics, the changelog, and
curated tutorial snippets.

## What changes
- **Tutorials mark featured blocks.** A region of a tutorial wrapped in
  `<!-- release-note: <title> -->` ... `<!-- /release-note -->` is a curated
  highlight. Because it lives in the tutorial, it stays verified and in sync; the
  author controls exactly what is featured.
- **A generator** `scripts/milestone-notes.sh <milestone-id>` writes
  `docs/releases/<slug>.md` for a milestone, assembling:
  - a header (milestone title + its definition of done),
  - **Highlights**: the marked `release-note` blocks pulled verbatim from the
    tutorials (the runnable examples),
  - **What shipped**: the milestone's completed child epics (titles), from beans,
  - **Changelog**: the current `## [Unreleased]` section of `CHANGELOG.md`.
  Deterministic output (sorted/stable) so it is reproducible.
- **A gate** `tests/check-milestone-notes.sh`: every **completed** milestone SHALL
  have a `docs/releases/<slug>.md` that **regenerates identically** (run the
  generator, diff against the committed file). This makes "the notes are current"
  an objective, CI-checkable golden test, and fails if a closed milestone has no
  notes or stale notes. It is chained into the docs gate.
- **First use:** mark featured blocks in `docs/TUTORIAL-FEATURES.md` and generate
  the notes for the "Road to v1" milestone when it is closed.

## Decisions (settled with the maintainer)
- **Curated marked regions** in tutorials (not whole-tutorial links, not
  live-run capture): the author features exact verified snippets.
- **A generator script + a golden gate** (not a hand-filled checklist): "current"
  is objective (regenerates identically), enforceable in CI.

## Non-goals
- Per-version (not per-milestone) release notes; `scripts/release.sh` +
  goreleaser already handle the per-tag GitHub release. This is the milestone
  narrative layer above that.
- Running the tutorial commands inside the gate (the snippets are already verified
  where they live; the gate checks the *notes* regenerate, not that nivis runs).
- Auto-closing milestones; closing a milestone stays a deliberate beans action.

## Impact
- New: `scripts/milestone-notes.sh` (generator), `tests/check-milestone-notes.sh`
  (golden gate, chained into `tests/check-docs-ssot.sh`), `docs/releases/`.
- `docs/TUTORIAL-FEATURES.md`: `release-note` markers around a few key blocks.
- `docs/RELEASING.md`: a "Closing a milestone" section documenting the flow.
- Tests: the gate itself is the test (golden regeneration); a generated
  `docs/releases/<road-to-v1>.md` committed as the first artifact.

Docs impact: new section; a "Closing a milestone" section in docs/RELEASING.md and
the generated docs/releases/ notes. No standalone new concept doc: it extends the
documented release process (per docs/DOCS-GATE.md). The release-note markers in
the feature tutorial are additive comments, not new prose.
