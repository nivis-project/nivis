---
# nixform2-oh90
title: nivis plan reports spurious '~ update' for a resource whose config depends on a datasource (datasource not read during plan)
status: completed
type: bug
priority: normal
tags:
    - discovered
created_at: 2026-06-19T15:46:43Z
updated_at: 2026-06-19T16:01:31Z
---

Found while running the features-0.4 tutorial. On a second 'nivis plan' of an already-applied stack, a resource whose config references a DATASOURCE output is reported as '~ update' even though nothing changed. In the tutorial, alpha.alpha_token.app (label = lookup.refAttr 'result', from data.alpha.alpha_lookup.existing) shows '~ update' on every re-plan.

## Root cause
internal/phase/driver.go PlanReport seeds the ledger ONLY from stored state (d.Store.List()). Datasources are never stored, so during plan the datasource data.alpha.alpha_lookup.existing has no value in the ledger. graph.ResolveTFTF then cannot fully resolve alpha.alpha_token.app (its label ref points at the unread datasource), so it is NOT in res.FullyKnown. The plan loop falls into the 'else if found' branch (~line 279-281) and reports OpUpdate ('in state but not resolvable this pass -> treat as an update pending'). So the spurious ~ is a side effect of not reading datasources during plan.

## Fix
PlanReport should read the (side-effect-free) datasources before/in the resolve loop and seed their outputs into the ledger, exactly as the apply Run loop does (readOne -> ledger.Append), so refs to datasources resolve and a dependent resource plans against the provider as a true no-op. Then alpha.alpha_token.app would correctly report '=' on a re-plan.

## Repro (hermetic)
--attr nivis.tutorial --var env=prod: apply once, then plan: alpha.alpha_token.app shows '~ update', beta.beta_record.app shows '=' (beta resolves because alpha is in state). The ~ is wrong; nothing changed.

Related: the apply-always-says-create reporting bug (same tutorial run).



## Resolution (2026-06-19, OpenSpec change plan-apply-output-fidelity archived as 2026-06-19-plan-apply-output-fidelity)

Both fixed together (one change: plan/apply output tells the truth). The state machine was already correct (verified: ids/values unchanged across re-applies, never a re-create); only the reporting was wrong.

- z57y (apply always + create): phase.AppliedNode gains an Op field; applyOne now returns its resolved plan.Op; cmd/nivis applyCmd renders each node by its op (+ create, ~ update, -/+ replace, = no-op), datasource stays r read.
- oh90 (plan spurious ~ update): PlanReport now reads the side-effect-free datasources into the ledger before resolving (iterating to a fixpoint), exactly as the apply loop does, so a datasource-dependent resource is FullyKnown and is planned against its provider -> reports its true op (= when unchanged) instead of the in-state-but-not-resolvable -> OpUpdate fallback.

Proof:
- Unit: internal/phase TestReplanDatasourceDependentReportsNoop (apply once -> OpCreate; re-plan -> OpNoop not OpUpdate; re-apply -> OpNoop with stored id/value unchanged).
- E2E: tests/e2e TestPlanApplyOutputFidelity drives the real nivis binary against nivis.tutorial: apply1 shows +; re-plan shows = (not ~) + 'No changes'; re-apply shows = (not +); NO_COLOR output is ANSI-free.

gofmt / go build ./... / go test ./... / nix tests / docs-ssot gate all green. CHANGELOG [Unreleased] notes both. Targets 0.4.4.
