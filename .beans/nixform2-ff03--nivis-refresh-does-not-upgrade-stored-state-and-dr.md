---
# nixform2-ff03
title: nivis refresh does not upgrade stored state, and drops the recorded schema version
status: todo
type: bug
priority: high
tags:
    - discovered
created_at: 2026-10-15T21:30:11Z
updated_at: 2026-10-15T21:30:11Z
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
