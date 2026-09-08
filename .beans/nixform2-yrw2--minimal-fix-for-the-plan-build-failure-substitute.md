---
# nixform2-yrw2
title: 'Minimal fix for the plan/__build failure: substitute at the remaining call sites'
status: scrapped
type: task
priority: low
tags:
    - discovered
created_at: 2026-09-08T14:27:31Z
updated_at: 2026-09-08T14:43:18Z
parent: nixform2-kovh
---

The minimal, targeted fix for `nixform2-hytv` (`plan` fails on any config with a `__build` leaf), recorded as the FALLBACK. The chosen path is the structural fix — OpenSpec change `substitute-build-leaves-at-resolve` — which fixes every affected path by construction. This bean exists so the cheap option is not lost if that change is abandoned, or if a patch release is needed before it can land.

## The minimal fix

`realiseValue` already substitutes a `__build` leaf with its path string, and `realiseBuild` short-circuits when `NoBuild` is set. So walking a resolved config with the no-build behaviour yields the path string while building nothing — the right semantics for a plan, which compares a path and does not care whether the artifact exists yet.

Add that walk at the sites that lack it:

- `PlanReport` → before `plan.Plan` (driver.go:335) — the reproduced failure
- `PlanReport` → before `readOne` (driver.go:275) — datasource configs
- `Run` → before `readOne` (driver.go:183) — datasource configs during apply
- `ResolveOutputs` → before `readOne` (driver.go:393) — datasource configs during `output`

Funnel all of them, and `applyOne`, through ONE helper. The defect is a forgotten call site; adding two more call sites without removing that failure mode invites the fifth.

## Why it is the fallback rather than the choice

It leaves the invariant implicit: "every config handed to a provider has been substituted" stays a thing each call site must remember. The structural fix moves substitution into `graph.ResolveTFTF`, where configs are already produced and where substitution is pure, and returns the list of builds for `applyOne` to realise — after which no call site can forget.

## When to prefer it

- The structural change is abandoned or stalls.
- A `0.6.1` is needed urgently and the structural change is not ready.

## Acceptance criteria (if taken)

- `plan` succeeds on a configuration with a `__build` leaf and reports the same diff as the corresponding apply.
- `plan` builds nothing: no derivation is realised.
- A datasource whose config carries a `__build` leaf works in `plan`, `apply`, and `output`.
- All five sites go through one helper, and a test asserts no `__build` leaf reaches a provider-bound config.

## Reasons for Scrapping

`substitute-build-leaves-at-resolve` shipped, moving substitution into
`graph.ResolveTFTF` and reporting the builds for the apply path to realise. All
five paths are covered by construction and an invariant test asserts it, so this
minimal alternative is no longer required. Kept for the record of what was
considered; scrap it if it is in the way.
