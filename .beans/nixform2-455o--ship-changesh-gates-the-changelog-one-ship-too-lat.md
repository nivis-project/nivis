---
# nixform2-455o
title: ship-change.sh gates the changelog one ship too late
status: todo
type: bug
tags:
    - discovered
created_at: 2026-09-24T15:59:10Z
updated_at: 2026-09-24T15:59:10Z
---

`tests/check-changelog.sh` only inspects ARCHIVED changes:

```
check-changelog.sh:57   for p in "$root"/openspec/changes/archive/*/proposal.md
```

`scripts/ship-change.sh` runs the docs checks at step 2 and archives at step 3:

```
==> [2/7] gate          <- check-docs-ssot.sh -> check-changelog.sh
==> [3/7] archive OpenSpec change
```

So the change being shipped is invisible to the check that is supposed to cover
it. Its `Changelog:` declaration is only ever verified on the NEXT ship, by which
time the commit is already pushed.

## How it surfaced

`configurable-s3-backend-encryption` (bean nixform2-rlbz) shipped on 2026-09-24
with a green gate and no `Changelog:` line in its proposal. The next change's gate
failed on it:

```
FAIL: .../archive/2026-09-24-configurable-s3-backend-encryption/proposal.md
      has no 'Changelog:' line
```

Fixed by hand while shipping `s3-backend-assume-role`, which also got its line
added up front so the same gap did not repeat. But the ordering is unchanged: the
gate still cannot see the change it is gating.

## Why it matters beyond the annoyance

The failure lands on the WRONG change. Whoever ships next gets a red gate for
someone else's omission, on a change they did not write, and the quickest way out
is to write a `Changelog:` line for work they may not know. That is how a gate
teaches people to satisfy it rather than to mean it.

It also means the guarantee the gate claims is not the one it provides: a change
can be committed and pushed with no changelog declaration at all.

## Fix

Either:

- have `check-changelog.sh` also inspect ACTIVE changes (`openspec/changes/*/`,
  not just `archive/*/`), so the change being shipped is covered by its own gate;
  or
- move the changelog check in `ship-change.sh` to run AFTER `openspec archive`,
  before the commit, so a missing declaration still aborts the ship.

The first is better: the check then works for anyone running it by hand, not only
inside the ship script. Note that an active change has no date prefix, so the
pre-convention CUTOFF exemption (`check-changelog.sh:28`, 2026-06-18) needs a
branch for the undated case.

Worth checking whether `check-docs-gate.sh` (the `Docs impact:` line) has the same
shape. It reported "27 in-scope change(s)" while the change being shipped was
still active, so it probably does cover active changes, but that should be
confirmed rather than assumed.
