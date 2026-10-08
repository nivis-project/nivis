---
# nixform2-msl4
title: Fake providers read Config where a real provider reads ProposedNewState
status: completed
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-10-08T22:02:21Z
updated_at: 2026-10-08T22:19:04Z
blocking:
    - nixform2-1mk0
---

Discovered while exploring nixform2-1mk0 on 2026-10-09.

All three fakes decode `ProposedNewState` only to test it for null (the delete
signal), then build the planned state from `Config`:

- `internal/fakeprovider/base.go:156`
- `internal/fakeproviderv5/base.go:106`
- `internal/fakeproviderx/fakeproviderx.go:130`

Same six lines three times. A real provider reads `ProposedNewState`.

## Why it has stayed invisible

It cancels out with nixform2-1mk0. `tfvalue.EncodeConfig` sends `unknown` in
`Config` for an attribute absent from config but computed in the schema, where
Terraform sends `null`. The fake echoes `Config`, so it plans `unknown` only
because the executor wrongly sent `unknown`. Two errors, one green suite.

Also: `Attr` has no `Default` field, so no fake can declare a schema default.
The construction that breaks in nixform2-1mk0 cannot be written in this
repository, which is why e2e never caught it.

And `decode` returns every attribute of the object type as a key, so the branch
that would write null for an absent attribute is unreachable for a well-formed
config object. The echo is total.

## Why it blocks nixform2-1mk0

Fixing the executor first shows up as a perpetual diff against the fakes, which
is indistinguishable from a real regression. Fixing both at once means a green
suite cannot tell "both correct" from "both wrong in a new compensating way",
which is the exact mechanism that produced the bug. The fakes go first.

The interim is safe: with the fakes fixed and the executor still wrong, a create
behaves as it does today and an update is slightly more correct, because the
fake sees prior merged in where it used to see unknown.

## OpenSpec

`fakes-model-the-plan-contract` (spec-driven; proposal, design, spec delta on
`fake-providers`, tasks). Validated, not yet implemented.


## Summary of Changes

All three fakes now build the planned state from `ProposedNewState` and use
`Config` only to answer whether the user set an attribute, which is the object
contract a real provider observes. `Attr` gained a `Default` seam, and
`delta_thing` declares one (`tier = "standard"`).

Precedence for an Optional+Computed attribute at plan:

1. unknown in config, so the user set an unresolved expression, plan unknown
2. known and non-null in config, take the configured value
3. a prior value carried in by `ProposedNewState`
4. the declared `Default`
5. unknown

Apply then keeps a computed attr already decided at plan and lets the Apply func
fill only the rest, so a default survives into the applied state.

### The decision that mattered

A first pass treated unknown-in-config as "unset", which made the default fire
even against today's encoder. That is more forgiving than
`terraform-plugin-framework`, which defaults only where config is null. It
would have made every test here green and meaningless, and nixform2-1mk0 could
not have been judged against the substrate. Corrected to the faithful rule and
recorded as design decision D5.

The visible consequence is the point: with the executor still sending unknown, a
declared default does NOT reach apply through a real spawn. That defect is now
pinned by `TestDeltaDefaultBlockedByEncoderUnknown`, whose failure message says
the next change inverts it. The honest substrate reproduces the bug it exists to
catch.

### Verification

`go test ./...` green, `tests/e2e` included, with no edit to e2e expectations
and `internal/tfvalue` untouched. The discriminating assertion was proved to
discriminate: reverting `planned` to read `Config` turns
`TestPlannedStateComesFromProposedNewState` and `TestPriorValueBeatsTheDefault`
red; restoring the fix turns them green.

Three in-process conformance files (one per fake, six tests each), two spawned
tests through the real `provider-delta` binary.

OpenSpec change: `fakes-model-the-plan-contract`.
