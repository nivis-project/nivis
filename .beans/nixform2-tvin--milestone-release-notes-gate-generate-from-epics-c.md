---
# nixform2-tvin
title: Milestone release-notes gate (generate from epics + changelog + tutorial highlights)
status: completed
type: feature
priority: normal
tags:
    - discovered
    - roadmap
created_at: 2026-06-18T12:36:19Z
updated_at: 2026-06-18T12:43:30Z
---

When a milestone closes, generate a release-notes document so the notes show what users can DO (concrete runnable examples), not just a changelog.

## Decision (with maintainer)
- Tutorials mark featured blocks with HTML comments `<!-- release-note: <title> -->` ... `<!-- /release-note -->`; the generator extracts those verified snippets into a "Highlights" section.
- A generator script scripts/milestone-notes.sh <milestone-id> assembles docs/releases/<slug>.md from: the milestone's completed epics (beans), the CHANGELOG [Unreleased] section, and the marked tutorial highlights.
- A gate (tests/check-milestone-notes.sh) verifies every COMPLETED milestone has a release-notes doc that regenerates identically (a golden check), so "current" is objective and CI-checkable.

First use: the "Road to v1" milestone (nixform2-zdj0), whose Phase A epics are all done. Mark featured blocks in docs/TUTORIAL-FEATURES.md.

OpenSpec change: milestone-release-notes (release spec delta + a docs note). Spec-before-code.


---
## Summary of Changes
DONE via OpenSpec change milestone-release-notes (archived 2026-06-18-milestone-release-notes):

- scripts/milestone-notes.sh <milestone-id>: generates docs/releases/<slug>.md from the milestone (goal/DoD + completed child epics from beans), the tutorials' release-note-marked blocks (Highlights), and the CHANGELOG [Unreleased] section. Deterministic (epics sorted by title, no timestamps).
- tests/check-milestone-notes.sh: a GOLDEN gate. For every completed milestone, the committed notes must regenerate identically (run the generator, diff). Missing/stale -> fail, naming the milestone + the generator. Pre-gate milestones (the PoC hj4w) are exempt (their tutorials predate the markers). Chained into check-docs-ssot.sh.
- Tutorials mark featured blocks with <!-- release-note: <title> --> ... <!-- /release-note -->; docs/TUTORIAL-FEATURES.md marks the phased-apply and the outputs blocks.
- docs/RELEASING.md: a "Closing a milestone" section documents the flow.
- First artifact: docs/releases/road-to-v1.md, generated for the now-completed Road-to-v1 milestone (zdj0). Verified: gate passes (current), and a stale edit / a missing file both fail it.

Also: completed the Road-to-v1 milestone (zdj0; all Phase A epics done) and reparented p4uz (gap 2, deferred) to the enterprise milestone so zdj0 could close cleanly.

DECISIONS (with maintainer): curated marked regions in tutorials (not whole-tutorial links or live-run capture); a generator script + golden gate (not a hand-filled checklist).
