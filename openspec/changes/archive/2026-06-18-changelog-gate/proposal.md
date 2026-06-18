# Proposal: changelog-gate

## Why
A user-facing change should not ship without a line in the changelog, but today
nothing connects an OpenSpec change being archived to `CHANGELOG.md`. The M1 batch
shipped ten archived changes with an empty `[Unreleased]` section until it was
written by hand. Tie the two together: when a change is archived, its changelog
status is recorded and enforced, the same way `Docs impact:` records the docs
decision.

## What changes
- **Each `proposal.md` carries a `Changelog:` line.** Either the entry text to add
  under `## [Unreleased]` (e.g. `Changelog: Added datasources (mkData) ...`), or
  `Changelog: none - <reason>` for an internal / non-user-facing change (a
  refactor, a gate, a doc-only tweak). This mirrors the existing `Docs impact:`
  convention.
- **A gate** `tests/check-changelog.sh`:
  - every archived change's `proposal.md` MUST have a `Changelog:` line;
  - for a line that is **not** `none`, a distinctive fingerprint of the entry text
    MUST appear in `CHANGELOG.md`'s `## [Unreleased]` section (so the declared
    entry is actually present before release);
  - `none`-marked changes need no changelog entry.
  It is chained into `tests/check-docs-ssot.sh`.
- **Pre-convention archives are exempt.** The 36 changes archived on or before the
  cutoff date predate this rule and are skipped (like the docs-coverage gate's
  cutoff); new archives must comply.

## Decisions (settled with the maintainer)
- **A declared `Changelog:` line in the proposal** (author decides
  user-facing-ness), not "every archive needs an entry" (noisy for internal
  changes) and not a reminder-only print (easy to forget).

## Non-goals
- Auto-writing the changelog from proposals (the author writes the entry; the gate
  checks it is present). Auto-generation could come later but invites wording the
  author would rewrite anyway.
- Per-version sectioning; `scripts/release.sh` already rolls `[Unreleased]` into a
  dated section at release time.

## Impact
- New: `tests/check-changelog.sh` (chained into `tests/check-docs-ssot.sh`).
- `docs/RELEASING.md`: document the `Changelog:` line + the gate.
- Active/new OpenSpec changes carry a `Changelog:` line going forward (this change
  itself does).
- The cutoff exempts existing archives; the M1 batch is already reflected in
  `[Unreleased]` (drafted alongside this), so the gate is satisfiable immediately
  for any post-cutoff change.

Changelog: none - tooling/process gate, no user-facing surface (the changelog
convention itself; nothing for an end user to see in a release).

Docs impact: new section in docs/RELEASING.md (the Changelog: line + the gate).
No new document: it extends the documented release process (per docs/DOCS-GATE.md).
