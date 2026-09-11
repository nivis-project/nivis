---
# nixform2-p72l
title: check-docs-gate.sh silently skips tinychange changes (no proposal.md)
status: todo
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-09-11T14:52:56Z
updated_at: 2026-09-11T14:52:56Z
---

Discovered while shipping the `evaluate-config-once-per-run` OpenSpec change
(bean nixform2-01kd), the first change in this repo to use the `tinychange`
schema.

## The gap

`tests/check-docs-gate.sh` collects its inputs with:

    proposals=("$root"/openspec/changes/*/proposal.md \
               "$root"/openspec/changes/archive/*/proposal.md)

A `tinychange` change has no `proposal.md` — its artifacts are `specs/` and
`tasks.md` only. Such a change therefore never enters the glob, is never
counted, and passes the gate **vacuously**. The script then reports
"ok: all N in-scope change(s) record a 'Docs impact:' decision" without having
looked at it.

This defeats the gate's stated purpose. From its own header:

> this script does NOT judge content. It enforces only that the judgment was
> RECORDED ... this guarantees the call was made and written down, never
> silently skipped.

A tinychange silently skips it. And a tinychange is arguably MORE likely to
need the nudge, not less: it is the format used when the work feels too small
to think hard about, which is exactly when docs get forgotten.

## Note on severity

A tinychange is small by definition, so the docs impact is usually "none" — but
"usually none" is the argument the gate exists to refuse. A small change can
still add a flag or alter an error message.

## Fix direction

Make the gate schema-aware rather than filename-driven: enumerate changes by
directory (or via `openspec list --json`) and look for the `Docs impact:` line
in whichever artifact that change's schema provides — `proposal.md` for
`spec-driven`, `tasks.md` for `tinychange`. Failing to find ANY artifact to
check should be a failure, not a skip, so a future schema cannot reopen the
same hole.

Keep the existing date-based exemption and the `change_is_ours` filter for the
shared store.

## Related

`nixform2-w74f` — the changelog gate cannot tell a misfiled entry. Same family:
a gate that checks presence but not placement/applicability.

The `evaluate-config-once-per-run` change records its decision in `tasks.md`
under a `Docs impact:` heading despite the gate not reading it, so the record
exists for when this is fixed.
