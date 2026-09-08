---
# nixform2-hytv
title: plan fails on any config with a __build leaf
status: completed
type: bug
priority: high
created_at: 2026-09-08T14:16:59Z
updated_at: 2026-09-08T14:55:04Z
parent: nixform2-kovh
---

`nivis plan` errors on any configuration containing a `__build` leaf:

```
error: encode config: attr "source": expected string, got map[string]interface {}
```

`apply` on the same configuration succeeds.

## Cause

`realiseBuilds` is called in exactly one place — `applyOne` (internal/phase/driver.go:480). `PlanReport` (driver.go:235) has no equivalent, so the raw `{"__build":{...}}` map reaches the provider encode step, which expects the string the leaf stands for.

## The fix is already available

`realiseValue` substitutes the leaf with its path string:

```go
return b.Path, nil // substitute the __build leaf with its path string
```

and `realiseBuild` short-circuits when `NoBuild` is set:

```go
if d.NoBuild {
    return nil // --no-build: use the path as-is (the provider errors if absent)
}
```

So walking the plan's resolved configs with the `NoBuild` behaviour gives plan the path string while building nothing. That is the right semantics: a plan compares a path, and whether the artifact exists yet is irrelevant to the diff. Building a multi-GB image to produce a plan would be wrong.

## Acceptance criteria

- `plan` succeeds on a configuration with a `__build` leaf and reports the same diff as the corresponding apply.
- `plan` builds nothing: no derivation is realised, no substituter is contacted.
- A test covers a plan over a `__build` leaf whose path is NOT valid in the store, which is the case that fails today.

## Impact

Affects every configuration that builds an artifact, so both known consumers:

- `nivis-demos` stack/020_vaultwarden_ec2 — found here; the demo can be applied but not planned.
- `infra` stack/020_core_host — same `drvFile` construction, so `plan` on that domain fails the same way. Its README only ever shows `apply` for that domain, which is likely why nobody noticed.

Worth noting the ordering: this is the second defect in the same area after nixform2-ebon. That one made apply impossible for an unsubstitutable derivation; this one makes plan impossible for any build at all. Both went unseen because the only production consumer applies without planning.

## Summary of Changes

Shipped as OpenSpec change `substitute-build-leaves-at-resolve` (archived in the
`nivis` store). The structural fix was taken; the minimal alternative
(`nixform2-yrw2`) is scrapped.

**Reproduced first**, on `nivis.buildProbe`:

```
$ nivis apply --attr nivis.buildProbe --var probe_token=…      ok
$ nivis plan  --attr nivis.buildProbe --var probe_token=…
error: encode config: attr "label": expected string, got map[string]interface {}
```

and after the fix:

```
$ nivis plan …
  = alpha.alpha_token.probe (alpha_token)
No changes. 1 resource(s) up to date.
```

**The scope was four broken paths, not one.** `realiseBuilds` was called from one
of five sites that hand a config to a provider, and `PlanReport` resolves twice:

| Path | before | after |
|--------------------------------------|-----------|-------|
| apply → resource (`applyOne`)        | works     | works |
| apply → datasource (`readOne`)       | **fails** | works |
| plan → datasource (`readOne`)        | **fails** | works |
| plan → resource (`plan.Plan`)        | **fails** | works |
| output → datasource (`readOne`)      | **fails** | works |

One encoder guards PlanResourceChange, ApplyResourceChange and ReadDataSource
alike, so a datasource leaf failed identically — in `apply` and `output` too, not
only `plan`. A resource plan failed only for a resource already in state (a plan
calls the provider only for those), which is why apply-then-plan was the repro
and why nobody hit it: the production consumers apply without planning.

**Not a regression from the previous fix.** Before `6e16a97` there was exactly one
`realiseBuilds` call site, and after it there was still one; what changed is that
apply started working on unbuilt derivations, which is what led to planning the
same config for the first time.

**What was built**

- `internal/graph/resolve.go`: `ResolveTFTF` substitutes every `__build` leaf with
  its output path in the configs it already deep-copies, and reports what it
  substituted per node in `ResolveResult.BuildOutputs` (sorted, with each leaf's
  derivation path). Substitution is a pure rewrite and does no I/O, which is what
  makes it safe on the paths that must not build. Datasource nodes live in the
  same graph, so they are covered by the same pass.
- `internal/phase`: `applyOne` realises from the reported list instead of walking
  the config; `realiseBuilds` lost its substitution duty. Building stays exactly
  where it was — per resource, in phase — so the fixpoint property is untouched.
- `nix/example/build-probe.nix` gained a datasource whose `query` is a `drv` leaf.
  No fake needed: `alpha_lookup` already takes a required string and echoes it.

**Why structural rather than adding the missing calls.** The defect *is* a
forgotten call site, and it was the second in this area. Substitution now happens
where configs are produced, so no path can produce a config that still carries a
leaf. The test asserts that property rather than the four symptoms: a recording
provider client captures every config handed over on all three RPCs across
`PlanReport`, `Run` and `ResolveOutputs`, and fails if a leaf survives. Verified
it catches the defect — removing substitution from the resolve pass makes it fail
on plan, apply and readDataSource, naming the exact attribute.

**The "plan builds nothing" test is not vacuous**: it seeds state with one probe
token, then plans a *different*, never-built token and asserts that probe's output
path is still invalid afterwards. The naive version would have passed by accident.

**Deliberately left out**: `plan` reporting the derivations an apply would build.
The `BuildOutputs` report is exactly that data, which is why it is tempting — but
it is new user-visible surface and this was a bugfix bound for a patch release. It
stays `nixform2-kjsh`, now cheap (noted there).

Coverage: `internal/graph` 82.0%, `internal/phase` 81.4% host / 77.2% sandbox;
overall 69.8% / 69.4%. Floors ratcheted (overall 68→69, phase 74→77).
