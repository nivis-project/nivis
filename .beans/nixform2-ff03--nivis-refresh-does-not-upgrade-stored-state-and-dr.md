---
# nixform2-ff03
title: nivis refresh does not upgrade stored state, and drops the recorded schema version
status: completed
type: bug
priority: high
tags:
    - discovered
created_at: 2026-10-15T21:30:11Z
updated_at: 2026-10-15T21:54:08Z
---

Found 2026-10-15 while exploring import (nixform2-3utj), one hour after
shipping `honour-provider-schema-versions` (commit a1f6650). It is a gap in
that change: `internal/refresh` is a separate flow from the phase driver, and I
fixed only the driver.

`internal/refresh/refresh.go`:

    refreshOne   client.Read(Stored: stored.Attrs)      no upgrade first
    Run:67       store.Set({ID, Type, Attrs: newAttrs}) SchemaVersion dropped

## Two defects

1. **No upgrade before the read.** `nivis refresh` feeds attributes written at
   an older schema version straight to a provider, which is exactly the bug
   nixform2-67gg describes, in a call site that change did not reach. The driver
   now calls `upgradeStored`; refresh does not.

2. **The recorded version is dropped on write.** Every refreshed entry loses its
   `schemaVersion`.

The second is accidentally harmless today: after a refresh the attributes really
are current, and absent means current, so it reads correctly. That is luck
rather than design, and it stops being luck the moment anything else writes a
version it means.

## Why it matters

`nivis refresh` is the command a user reaches for when they suspect state is
stale, which is precisely when a provider has been upgraded. The one path where
the upgrade matters most is the one that skips it.

## Fix direction

Give `internal/refresh` the same comparison the driver has. The logic lives in
`Driver.upgradeStored` and should move somewhere both can use it rather than
being copied, since this bean exists because it was in one place and needed in
two.

Set `SchemaVersion` explicitly on the write, to the version the schema reports,
rather than letting it default to zero and relying on absent-means-current.

## Check while there

Whether any other path writes `state.ResourceState` without the version.
`internal/apply` sets it; the driver's replace path deletes rather than writes.
A grep for `state.ResourceState{` is the audit.


## Summary of Changes

`Driver.upgradeStored` became `plan.UpgradeStored`, beside `SchemaFor`,
which was already the precedent for a helper both the driver and refresh use. It
takes the node plus the stored version and attributes, so `internal/plan`
gains no dependency on `internal/state`. The driver's schema-version tests
pass untouched, which is what proves the relocation was a relocation.

`internal/refresh` now upgrades before reading, refuses a recorded version
newer than the provider's, and records the current version explicitly rather
than letting it default to zero.

### The part that was specified rather than discovered

Inserting the upgrade without moving `state.Drifted` would have introduced a
NEW defect in the same change that fixed this one: comparing the pre-upgrade
attributes against a fresh read reports the provider's own rename as
infrastructure having changed underneath the user. The design called it out
because it is invisible until the code is written, and the test fails loudly if
anyone moves the comparison back: `drift reported for [p.t.r]` for a resource
nothing had touched.

### Verification

All three key tests proved to discriminate. With the upgrade removed,
`upgrade calls = 0, want 1` and the refusal does not fire. With drift compared
pre-upgrade, the false alarm appears.

Six new tests beside six pre-existing ones, all passing, none edited.
`tests/` untouched. The audit this bean asked for still shows exactly two
writers of `state.ResourceState`, and both now record a version, so the pair
is closed.

OpenSpec change: `refresh-honours-schema-versions`.
