# Spec delta: release

## ADDED Requirements

### Requirement: Milestone release notes are generated and gated
When a milestone closes, the project SHALL have a generated release-notes document
for it under `docs/releases/`, assembled deterministically from three sources by a
generator (`scripts/milestone-notes.sh <milestone-id>`):
- **Highlights**: blocks marked in the tutorials with
  `<!-- release-note: <title> -->` ... `<!-- /release-note -->`, pulled verbatim
  (the verified, runnable examples).
- **What shipped**: the titles of the milestone's completed child epics, from the
  beans tracker.
- **Changelog**: the current `## [Unreleased]` section of `CHANGELOG.md`.

A gate SHALL enforce that every **completed** milestone has such a document and
that the document **regenerates identically** (running the generator and diffing
against the committed file produces no difference), so stale or missing milestone
notes fail the check. The gate SHALL run as part of the docs checks. Generation
SHALL be deterministic (stable ordering, no timestamps that change per run) so the
golden comparison is reliable.

#### Scenario: a closed milestone has regenerable notes
- GIVEN a milestone marked completed in beans
- WHEN the milestone-notes gate runs
- THEN it finds `docs/releases/<slug>.md`, regenerates it, and the regenerated content matches the committed file.

#### Scenario: missing or stale notes fail the gate
- GIVEN a completed milestone whose release-notes doc is absent or no longer matches what the generator produces
- WHEN the gate runs
- THEN it fails, naming the milestone and pointing at `scripts/milestone-notes.sh`.

#### Scenario: highlights come from marked tutorial blocks
- GIVEN a tutorial with a `release-note`-marked block
- WHEN the notes are generated
- THEN that block appears verbatim under Highlights in the milestone's notes.
