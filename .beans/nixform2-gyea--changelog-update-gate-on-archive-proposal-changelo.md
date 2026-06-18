---
# nixform2-gyea
title: 'Changelog-update gate on archive (proposal Changelog: line, reflected in [Unreleased])'
status: completed
type: feature
priority: normal
tags:
    - discovered
    - roadmap
created_at: 2026-06-18T16:00:39Z
updated_at: 2026-06-18T16:05:24Z
---

A user-facing change should not ship without a changelog note. Tie the changelog to OpenSpec archival.

## Decision (with maintainer)
- Each proposal.md carries a `Changelog:` line: the entry text, OR `none - <why>` for internal/non-user-facing changes (mirrors the `Docs impact:` convention).
- A gate (tests/check-changelog.sh) checks: every archived change has a `Changelog:` line; for non-`none` lines, the entry text (a fingerprint) appears in CHANGELOG.md [Unreleased]. Chained into check-docs-ssot.sh.
- Pre-convention archived changes (everything archived on/before 2026-06-18) are exempt by date prefix, like the docs-gate cutoff.

OpenSpec change: changelog-gate (release spec delta). Spec-before-code.


---
## Summary of Changes
DONE via OpenSpec change changelog-gate (archived 2026-06-18-changelog-gate):
- CONVENTION: every proposal.md carries a `Changelog:` line - the entry text added under CHANGELOG.md [Unreleased], or `none - <reason>` for internal changes (mirrors `Docs impact:`).
- GATE: tests/check-changelog.sh - every archived change has the line; a non-`none` entry must appear in [Unreleased] (matched after normalizing markdown markup + whitespace, so bold/code/wrapping are tolerated). Pre-convention archives (<= 2026-06-18) exempt by date. Chained into check-docs-ssot.sh.
- docs/RELEASING.md documents it.
- Verified all four cases (declared-and-present passes; declared-but-absent fails; none passes; missing line fails) and that archiving this change keeps the gate green.

Also drafted the M1 CHANGELOG [Unreleased] section (the Road-to-v1 batch). Notably the milestone-notes golden gate then flagged the road-to-v1 release notes as stale (they embed the changelog) - regenerated them, so changelog and release notes stay consistent. The two gates compose.
