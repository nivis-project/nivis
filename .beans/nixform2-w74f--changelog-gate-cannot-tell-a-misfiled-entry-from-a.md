---
# nixform2-w74f
title: Changelog gate cannot tell a misfiled entry from a present one
status: todo
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-09-08T13:22:52Z
updated_at: 2026-09-08T13:22:52Z
---

`tests/check-changelog.sh` cannot tell "the entry is present" from "the entry is filed under the right version", so a changelog entry can land in an already-released section and the gate still passes.

## How it was found

While preparing the release after `realise-build-leaves-from-drv`, the `__build` fix's entry turned out to be inside the released `## [0.5.0]` section rather than `## [Unreleased]`. The entry had been added by anchoring on the text of a neighbouring `### Fixed` bullet — which, after `release.sh` rolled the previous Unreleased into 0.5.0, existed only in the released section. The gate passed because it searches the whole changelog.

## Why the gate is written that way

Deliberately, per its own comment: a change archived BEFORE a release and read AFTER it would no longer have its entry in `[Unreleased]` (release.sh moved it into the version section), so a strict "must be in Unreleased" rule would fail for legitimately-released work.

## A rule that would catch it

Require the entry to be in `[Unreleased]` when the change was archived AFTER the date of the most recent released version (the top `## [x.y.z] - <date>` heading), and fall back to the whole-file search only for changes archived on or before that date. Both facts are already available: the archive directory carries a date prefix, and the changelog carries the release date.

## Acceptance criteria

- An entry for a change archived after the last release, filed outside `[Unreleased]`, fails the gate and names both the change and where the entry was found.
- An entry for a change archived before the last release still passes when its entry sits in the version section that shipped it.
- The false-negative that motivated this (an entry in a released section for work that came later) is covered by a fixture or a self-test.
