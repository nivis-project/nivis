---
# nixform2-mbp1
title: Fake providers cannot signal RequiresReplace, so no test drives a replacement
status: completed
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-10-09T08:23:11Z
updated_at: 2026-10-09T08:47:16Z
blocking:
    - nixform2-aur6
---

Discovered while exploring nixform2-aur6 on 2026-10-09.

`grep -rn RequiresReplace` over `internal/fakeprovider`,
`internal/fakeproviderv5` and `internal/fakeproviderx` returns nothing. No fake
can tell the executor a change requires replacement, so the executor's
replacement branch is never driven through the protocol by any test.

The only replacement coverage is `internal/phase/replace_test.go`, whose
`recordClient` is a hand-rolled stub at the `provider.Client` level, above the
msgpack encoding. It can prove which prior the driver passed; it cannot prove
what the planned state contained. The defect in nixform2-aur6 happens below it,
in `ProposedMsgPack`.

Two gaps, both needed:

- A fake cannot return `RequiresReplace` (`[]*tftypes.AttributePath` on both
  tfprotov5 and tfprotov6), and does not decode `PriorState` at plan at all, so
  it cannot tell a create from a change to an existing object.
- No fake resource has an optional-computed attribute with no default that the
  provider fills at apply. That is the construction that leaves a value in prior
  state which the configuration never set, which is what a replacement wrongly
  inherits. It is the shape of the AWS `security_groups` attribute in the
  original report.

## Why this blocks nixform2-aur6

Same shape as nixform2-msl4 blocking nixform2-1mk0. Fixing the executor against
doubles that cannot reproduce the defect gives a green suite that proves
nothing. The substrate goes first, keeps the suite green on its own, and leaves
the defect reproducible so the fix can be judged.

## OpenSpec

`fakes-can-force-a-replacement` (spec-driven; proposal, design, spec delta on
`fake-providers`, tasks). Validated, not implemented.


## Summary of Changes

All three fakes now decode `PriorState` at plan and can return
`RequiresReplace`. `Attr` gained a `ForceNew` marker; a marked attribute
whose configured value differs from the prior value is reported in
`RequiresReplace`, as a real provider does.

Two guards, both tested in all three fakes:

- A null prior is a create and never requires replacement.
- An attribute absent from config is not a change. Treating it as one would make
  every unset optional force-new attribute force a replacement on every plan.

`delta_thing` gained `image` (optional, force-new) and `group` (optional,
computed, no default, filled by the provider at apply). Together they model the
AWS report: the replacement is forced by one attribute while a different,
provider-filled one is the thing that leaks.

### What this makes visible

`TestDeltaForceNewInputIsReplaced` drives the real `provider-delta` binary:
create once so `group` takes a provider-chosen value, change `image`, plan
against the stored object. The plan reports a replacement, and `group` is NOT
reported known-after-apply. It is inherited from the object about to be
destroyed, where a create would leave it unknown.

That is nixform2-aur6, reproducible through a spawned provider for the first
time. The test asserts the defect, and its failure message tells whoever lands
the fix to invert it.

### Verification

`go test ./...` green including `tests/e2e`, with no edit to any existing
expectation and `tests/` unedited. `internal/plan`, `internal/phase`,
`internal/tfvalue` and `internal/provider` are untouched, so the next change
has something to prove.

Three conformance files gained the same five force-new cases; the v6 fake also
covers the provider-filled attribute reaching prior state.

OpenSpec change: `fakes-can-force-a-replacement`.
