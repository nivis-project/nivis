---
# nixform2-pex6
title: check-changelog.sh can only fail after a change is archived, and its matching rule is a trap
status: todo
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-09-14T13:56:47Z
updated_at: 2026-09-14T13:56:47Z
---

Discovered while shipping `stream-run-progress` (bean nixform2-7gdt) and cutting
v0.7.0 on 2026-09-14. Caught by reading `docs/RELEASING.md` before the release,
not by any gate.

## What happened

`openspec/changes/stream-run-progress/proposal.md` was written without a
`Changelog:` line. Every gate passed. The change was one step from being
archived, after which the NEXT gate run — the release I was about to cut —
would have failed.

Two independent problems combined.

## Problem 1: the gate cannot fail while the mistake is still cheap

`tests/check-changelog.sh` collects its inputs from ARCHIVED changes only:

    for p in "$root"/openspec/changes/archive/*/proposal.md; do

An ACTIVE change is never inspected. So the sequence is:

    write proposal without Changelog:   -> gate passes (not archived yet)
    implement, test, run the full gate  -> gate passes (not archived yet)
    ship: gate runs, then archives      -> gate passes, THEN archives
    any later run (e.g. the release)    -> FAILS

The gate reports green at every point where fixing it is cheap, and red only
once the change is already archived and pushed. `tests/check-docs-gate.sh` has
the same structure (it globs `changes/*/proposal.md` AND the archive, so it is
less bad — it does see active changes).

Note this is the same SHAPE as nixform2-p72l (the docs gate skipping
tinychanges): a gate that checks for presence at a moment when the thing cannot
yet be wrong.

## Problem 2: the matching rule punishes the natural phrasing

The check fingerprints the FIRST FIVE WORDS of the `Changelog:` value and
requires them, normalized, as a literal substring of `CHANGELOG.md`:

    fingerprint="$(printf '%s' "$value" | normalize | cut -d' ' -f1-5)"

`normalize` strips `* _ ` #` and collapses whitespace, but does NOT strip the
`-` of a list bullet.

So the obvious thing to write —

    Changelog: Added `--log-level` to control how much progress is reported.

— produces the fingerprint `added --log-level to control how`, which does NOT
appear in a CHANGELOG whose entry is:

    ### Added
    - `--log-level` ...

because the normalized body reads `added - --log-level ...` (bullet retained)
and the category word is a heading, not part of the entry. The convention that
actually works is to omit the category word entirely and start with the entry
text, which is what the archived examples do:

    Changelog: Provider log lines are rendered as one readable note each ...

Nothing states this. `docs/RELEASING.md` shows `Changelog: Added datasources
(nivis.mkData): ...`, which is the phrasing that FAILS.

## Fix direction

1. Make the gate inspect ACTIVE changes as well as archived ones, so the
   failure lands while the change is still being written. Keep the date-based
   exemption and the `change_is_ours` filter.
2. Either make the matcher tolerant of a leading category word
   (`Added`/`Changed`/`Fixed`/`Removed`) and of the list bullet, or state the
   convention explicitly in `docs/RELEASING.md` and fix its example, which
   currently demonstrates the failing form.
3. Consider reporting the computed fingerprint on success at `-v`, so an author
   can see what will be matched rather than discovering it from a failure.

## Related

- `nixform2-p72l` — `check-docs-gate.sh` silently skips tinychange changes.
- `nixform2-w74f` — the changelog gate cannot tell a misfiled entry.

All three are the same family: presence checks that are green at the moment
they should be red.
