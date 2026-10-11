---
# nixform2-cjxz
title: Fake requires-replace compares config to prior, where a provider compares planned to state
status: completed
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-10-10T13:34:23Z
updated_at: 2026-10-11T10:13:10Z
blocking:
    - nixform2-nwnf
---

Discovered while exploring nixform2-nwnf on 2026-10-10, and introduced by my own
change `fakes-can-force-a-replacement` (nixform2-mbp1, commit 9b2a6c2).

terraform-plugin-framework decides requires-replace in
`resource/schema/stringplanmodifier/requires_replace_if.go`:

    if req.State.Raw.IsNull()              { return }   // not a create
    if req.Plan.Raw.IsNull()               { return }   // not a destroy
    if req.PlanValue.Equal(req.StateValue) { return }   // unchanged
    resp.RequiresReplace = true

It compares the PLANNED value against STATE. The fakes compare CONFIG against
PRIOR, and skip a config value that is null or unknown
(`internal/fakeprovider/resource.go`, and the two copies).

Two divergences follow:

- An unknown planned value is not equal to a known state value, so a provider
  replaces. The fakes skip unknown, so they do not. This is exactly the
  behaviour a plan-time unknown-propagation cascade depends on, so that work
  cannot be tested against this substrate.
- A force-new attribute removed from config plans as null against a known
  state value, so a provider replaces. The fakes skip null config, so they do
  not.

## The reasoning that was wrong

Design decision D2 of `fakes-can-force-a-replacement` justified skipping null
config as preventing "every unset optional force-new attribute forcing a
replacement on every plan". That is not a real risk when comparing the planned
value: `ProposedNew` already carries prior into unset computed attributes, so
plan equals state and nothing fires. The guard solved a problem the correct
comparison does not have.

## Severity

The fakes only ever UNDER-report requires-replace, never over-report. Nothing
shipped behaves wrongly because of this and no existing test is invalidated.
The cost is that the substrate cannot express two real provider behaviours.

## Fix direction

Compare the planned state the fake is about to return against the prior, rather
than config against prior, in all three fakes. Then re-check the conformance
tests added in nixform2-mbp1: the "absent from config is not a change" case
changes meaning and must be re-derived rather than kept.


OpenSpec change: `fake-replace-compares-planned-to-state` (spec-driven;
proposal, design, MODIFIED spec delta on `fake-providers`, tasks). Validated,
not implemented.


## Summary of Changes

All three fakes decide requires-replace by comparing the PLANNED value against
prior, with no null or unknown exclusion. A shared `valueAt` treats an absent
key as null, so a missing entry and an explicit null compare equal.

Two behaviours the substrate could not express before, now tested in each fake:

- an unknown planned value against a known prior replaces, which is what a
  cascade through an unresolved reference depends on (nixform2-nwnf);
- a force-new attribute removed from config replaces, its planned value being
  null against a known prior.

The old "absent from config is not a change" scenario was deleted rather than
kept: it stated the right outcome for the wrong reason. Its real content is now
`TestUnsetOptionalComputedForceNewDoesNotRequireReplace`, which passes with no
exclusion at all, because the planned value carries prior forward.

### Two harness defects found while implementing

`planReplace` passed `cfg` as `ProposedNewState`. The executor passes a
MERGED proposed, where a computed attr the config does not set carries its prior
value. Without the merge an unset optional-computed attr plans unknown and
differs from prior on every plan, so two tests failed for a reason unrelated to
the rule. Fixed to build a realistic proposed object.

Priors were also built with the computed attr null. A computed attr in state
holds a value; a null one makes an unknown planned value differ from it for
reasons the test is not about. Priors now carry the provider-chosen value.

Both are the same class of mistake as the one this bean reports: a double that
does not model what the real caller sends.

### Verification

`go test ./...` green including `tests/e2e`, with `tests/` and `cmd/`
unedited and no executor package touched. Nothing moved: since the fakes
previously under-reported, a test that moved would have been relying on a
replacement not being detected.

OpenSpec change: `fake-replace-compares-planned-to-state`.
