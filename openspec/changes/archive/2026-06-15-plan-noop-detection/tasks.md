# Tasks: plan-noop-detection

## 1. Spec
- [x] 1.1 Write proposal, tasks, executor spec delta (MODIFIED plan/apply: detect + skip no-ops)
- [x] 1.2 `openspec validate plan-noop-detection` passes

## 2. Implement
- [x] 2.1 `provider.PlanResult` gains `NoOp`
- [x] 2.2 v5/v6 Plan: when prior present, decode planned + compare to prior; equal & no unknowns & no replace => NoOp
- [x] 2.3 `plan.Plan`: add `OpNoop`; render `=` / unchanged; choose NoOp over Update when the backend says NoOp
- [x] 2.4 `applyOne`: on NoOp, skip Apply (and Destroy); seed the ledger with the prior attrs so dependents resolve
- [x] 2.5 `tn plan`: plan each resource against prior state; mark `+`/`~`/`-/+`/`=`; report actual change count

## 3. Verify + close
- [x] 3.1 Unit: prior==planned -> NoOp; applyOne calls neither Apply nor Destroy on NoOp
- [x] 3.2 Live AWS: apply the S3 example; `tn plan` shows no changes; a second `tn apply` is a no-op (no 404); destroy clean
- [x] 3.3 Full gate (build, go test, nix, IR conformance); gofmt
- [x] 3.4 `openspec archive plan-noop-detection`; fold into executor spec
- [x] 3.5 Close beans-l2q2; commit as Pim Snel; push
