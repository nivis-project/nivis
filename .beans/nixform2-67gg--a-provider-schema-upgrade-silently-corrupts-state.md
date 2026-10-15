---
# nixform2-67gg
title: 'A provider schema upgrade silently corrupts state: no version recorded, UpgradeResourceState never called'
status: completed
type: bug
priority: high
tags:
    - discovered
created_at: 2026-10-15T20:46:19Z
updated_at: 2026-10-15T21:07:52Z
blocking:
    - nixform2-3utj
---

Found 2026-10-15 while sizing the import work (nixform2-3utj). It is not an
import gap: it bites anyone who upgrades a provider.

## What is missing

nivis reads `Schema.Version` off the wire in `GetProviderSchema` and drops it:
`provider.ResourceSchema` is `{TypeName, Attrs, Blocks, Raw}`. State is
`{id, type, attrs}` with no version. `UpgradeResourceState` is never called
anywhere in the executor.

## What happens on a provider upgrade

`tfvalue.EncodeState` resolves any mismatch between stored attrs and the current
schema silently:

| schema change        | what nivis does                                        |
|----------------------|--------------------------------------------------------|
| attribute renamed    | old name dropped, new name null: a change, or a replace, out of nothing |
| attribute removed    | dropped silently (benign)                              |
| attribute added      | null (usually benign)                                  |
| attribute type changed | hard error naming the attribute, not the cause       |

The first row is the dangerous one. A provider renaming an attribute in a major
version makes nivis propose destroying live infrastructure, and the message will
not mention the provider version.

## What OpenTofu does

`internal/tofu/upgrade_resource_state.go`, three behaviours nivis has none of:

    71   state version == current, JSON attrs  -> skip the call entirely
    77   state version >  current              -> REFUSE
             "created by a newer provider version than is currently selected.
              Upgrade the provider to work with this state."
    95   otherwise                             -> UpgradeResourceState(state's version)

Line 98: even when versions match it still calls the provider "for any other
minor fixups" when the fast path does not apply.

The fast path matters for cost. In a nivis-only lifecycle where the provider
never changes, the call is skipped, so this is free in the common case.

## The format question has a clean answer

`document` is `{resources: map[id]ResourceState}` with no document-level
version, and `parseDocumentStrict` uses plain `json.Unmarshal` with no
`DisallowUnknownFields`. So adding `schemaVersion` with `omitempty` is backward
compatible (old documents parse) AND forward compatible (an older nivis ignores
the field rather than rejecting the document, which matters for state
pull/push across versions). The executor spec already grants the freedom: "The
format is internal (no tfstate compatibility)".

The real decision is what a MISSING version means:

- 0 is wrong and dangerous. State written today by a provider at version 2 has
  no field; reading it as 0 asks the provider to upgrade 0 -> 2 on state that is
  already current.
- "assume current" preserves today's behaviour exactly for every existing
  document, and only newly written state carries a real version. The upgrade
  path starts working from the next apply onward.

Take "assume current", and say so in the spec rather than leaving it to a zero
value.

## Why import needs it

A Terraform state document carries `schema_version` per instance. Import can
read and record it, after which nivis can upgrade when the configured provider
is newer. Without the field, imported Terraform state is silently assumed to
match whatever provider happens to be configured, which is exactly the "looks
like it worked, first apply destroys production" failure nixform2-3utj exists to
prevent.

## Known substrate gap

`postgresql_role` has no `SchemaVersion` and no `StateUpgraders`, so the
nixform2-jkeh harness cannot test this. A fake that declares a schema version
and an upgrader is needed first. Same pattern as nixform2-msl4, mbp1 and cjxz.


OpenSpec change: `honour-provider-schema-versions` (spec-driven; proposal,
design, spec deltas on `executor` and `fake-providers`, tasks). Validated,
not implemented.


## Summary of Changes

`provider.ResourceSchema` carries the schema version, captured in both
backends. `state.ResourceState` records it. Before a stored object is used as
prior state the two are compared: equal makes no call, older calls the
provider's upgrade, newer is refused.

A new `UpgradeState` on `provider.Client`, implemented by v5 and v6. nivis
sends the JSON side of `RawState` and never flatmap: it has never written
flatmap state.

### The decision that made it safe

A missing `schemaVersion` means CURRENT, not zero. Every fake declares
`Version: 1` today, so state written now was written at 1; reading it as 0
would ask providers to upgrade already-current state. Assuming current leaves
every existing document behaving exactly as before, which is the bar for
changing a state format in a tool that manages live infrastructure.

### Substrate first, again

The fakes could not express a schema change: they declared `Version: 1`
hardcoded and their `UpgradeResourceState` decoded at the CURRENT object type
and re-encoded, which only works when nothing changed. They now declare a
version selectable at run time (`TERRAE_NIVIS_FAKE_SCHEMA_VERSION`, unset
keeps 1) and carry declared renames, decoding the raw JSON themselves and
applying the transformation for the version the REQUEST names.

### A stub that passed for the wrong reason

`TestRenamedAttributeDoesNotBecomeAChange` passed with the comparison
reverted, because its stub reported a no-op for any non-nil prior regardless of
whether the state had been upgraded. Fixed to notice. Only then did reverting
produce `resource resolved as 1` (OpUpdate), which is the change-out-of-nothing
the bug produces.

Third time this session the falsification step caught a test that could not
fail.

### Verification

`go test ./...` green. Both directions discriminate. A document carrying the
field restores; one without round-trips without gaining it; an unknown field is
ignored rather than rejected, which is what lets an older nivis read what this
one writes. The real-provider VM test passes, and since `postgresql_role`
declares no schema version it exercises the missing-version path against a
genuine third-party provider.

Widening `provider.Client` broke six test stubs: a compile break, not a moved
expectation. Each gained an identity implementation.

OpenSpec change: `honour-provider-schema-versions`.
