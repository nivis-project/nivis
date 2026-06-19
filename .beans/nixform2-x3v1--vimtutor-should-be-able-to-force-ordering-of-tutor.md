---
# nixform2-x3v1
title: 'nivistutor: getting-started always first in the tutorial menu (controlled ordering)'
status: completed
type: task
priority: normal
created_at: 2026-06-19T15:37:03Z
updated_at: 2026-06-19T16:20:58Z
---

getting started should always be first



## Detail
nivistutor's menu/--list currently sorts tutorials alphabetically (listTutorials in cmd/nivistutor/tutorials.go), which puts 'features-0.4' BEFORE 'getting-started'. getting-started is the entry-point tutorial and should always be first; per-release feature tutorials follow it.

## Desired order
1. getting-started (always first)
2. the rest, features-<version> newest version first (so the most recent release's tutorial is most prominent), then any others alphabetically as a stable tiebreak.

## Acceptance
- nivistutor --list and the interactive menu show getting-started first.
- Adding a new features-<version> tutorial slots in by version without code changes.
- A unit test asserts the order (getting-started first; features newest-first).
- go test ./... + nix tests + docs-ssot gate green; CHANGELOG [Unreleased] notes it.

OpenSpec change: nivistutor-menu-order.



## Resolution (2026-06-19, OpenSpec change nivistutor-menu-order archived as 2026-06-19-nivistutor-menu-order)

cmd/nivistutor/tutorials.go listTutorials now orders via lessTutorial: getting-started first, then features-<version> newest-version-first (numeric compare via parseVersion/compareVersion, so 0.10 > 0.5 > 0.4), then any others alphabetically. A new features-<version> slots in by version with no code change. Unit tests: TestTutorialOrder (synthetic 0.4/0.5/0.10 + extras), TestEmbeddedMenuGettingStartedFirst, TestCompareVersionNumeric. --list and the interactive menu now show getting-started first (was alphabetical, which put features-0.4 first). go test ./... + nix tests + docs-ssot gate green; CHANGELOG [Unreleased] notes it. Targets 0.4.4.
