---
# nixform2-z57y
title: nivis apply reports every resource as '+ create' regardless of the real op (update/noop/replace)
status: completed
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-06-19T15:46:43Z
updated_at: 2026-06-19T16:01:31Z
---

Found while running the features-0.4 tutorial. On a second 'nivis apply' of an already-applied stack, the apply output prints '+ create' for every resource even when the resource was updated in place or was a no-op. The underlying state machine is correct (verified: ids and computed values are unchanged across applies, so it is NOT re-creating), but the reporting lies.

## Root cause
cmd/nivis/main.go applyCmd (~line 207-215) renders every AppliedNode with out.create(n.ID, ''). The phase.AppliedNode struct (internal/phase/driver.go ~line 94) does not carry the resolved plan.Op, so the renderer cannot distinguish create/update/replace/noop. applyOne already computes pr.Op (OpCreate/OpUpdate/OpReplace/OpNoop) and acts on it correctly; it just is not surfaced to the reporter.

## Fix
Thread the real plan.Op out of applyOne into AppliedNode, and have applyCmd render it with the same vocabulary as plan (+ create, ~ update, -/+ replace, = no change). A datasource stays 'r read'. Consider: an OpNoop resource should arguably not be reported as an applied change (or reported as '=').

## Repro (hermetic)
Scaffold/observe the features-0.4 tutorial (--attr nivis.tutorial --var env=prod): apply twice; the second apply prints '+ create' for alpha.alpha_token.app and beta.beta_record.app though nothing is created (state ids unchanged: alpha-0).

Related: nixform2-oh90 (plan reports spurious ~ update for a datasource-dependent resource); both are plan/apply reporting bugs surfaced by the same tutorial run. They are best fixed together (one OpenSpec change), since both are about plan/apply output telling the truth.



## Resolution (2026-06-19, OpenSpec change plan-apply-output-fidelity archived as 2026-06-19-plan-apply-output-fidelity)

Both fixed together (one change: plan/apply output tells the truth). The state machine was already correct (verified: ids/values unchanged across re-applies, never a re-create); only the reporting was wrong.

- z57y (apply always + create): phase.AppliedNode gains an Op field; applyOne now returns its resolved plan.Op; cmd/nivis applyCmd renders each node by its op (+ create, ~ update, -/+ replace, = no-op), datasource stays r read.
- oh90 (plan spurious ~ update): PlanReport now reads the side-effect-free datasources into the ledger before resolving (iterating to a fixpoint), exactly as the apply loop does, so a datasource-dependent resource is FullyKnown and is planned against its provider -> reports its true op (= when unchanged) instead of the in-state-but-not-resolvable -> OpUpdate fallback.

Proof:
- Unit: internal/phase TestReplanDatasourceDependentReportsNoop (apply once -> OpCreate; re-plan -> OpNoop not OpUpdate; re-apply -> OpNoop with stored id/value unchanged).
- E2E: tests/e2e TestPlanApplyOutputFidelity drives the real nivis binary against nivis.tutorial: apply1 shows +; re-plan shows = (not ~) + 'No changes'; re-apply shows = (not +); NO_COLOR output is ANSI-free.

gofmt / go build ./... / go test ./... / nix tests / docs-ssot gate all green. CHANGELOG [Unreleased] notes both. Targets 0.4.4.
