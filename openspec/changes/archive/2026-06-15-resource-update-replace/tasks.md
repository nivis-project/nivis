# Tasks: resource-update-replace

## 1. Spec
- [x] 1.1 Write proposal, tasks, executor spec delta (MODIFIED plan/apply/client; ADDED create-vs-update-vs-replace)
- [x] 1.2 `openspec validate resource-update-replace` passes

## 2. Protocol/client layer
- [x] 2.1 `provider.PlanRequest` gains `PriorState` (nil = create); `provider.PlanResult` gains `RequiresReplace`
- [x] 2.2 `provider.ApplyRequest` gains `PriorState`
- [x] 2.3 v5 backend: encode prior state (EncodeState) when present, else priorNull; read `GetRequiresReplace()` into PlanResult
- [x] 2.4 v6 backend: same as v5

## 3. Plan/apply decision
- [x] 3.1 Plan engine / phase driver: `store.Get(id)` → thread prior attrs into PlanRequest
- [x] 3.2 Apply: no prior → create; prior + no replace → update in place; prior + replace → destroy-then-create (reuse destroy path), no orphan
- [x] 3.3 Plan rendering: distinguish create (`+`), update (`~`), replace (`-/+`)

## 4. Tests
- [x] 4.1 Fake-provider: update-in-place (normal attr change re-applies without recreate)
- [x] 4.2 Fake-provider: replace (a force-new attr → destroy+create; old gone)
- [x] 4.3 Plan shows create/update/replace correctly
- [x] 4.4 Gated AWS apply-twice e2e: in-place tag change; `bucket` change forces replace; exactly one bucket remains, no orphan
- [x] 4.5 Full gate: go build, go test, nix tests, IR conformance

## 5. Close out
- [x] 5.1 `openspec archive resource-update-replace`; fold into executor spec
- [x] 5.2 Close beans-faaf; update the lifecycle epic (beans-ft9v); commit as Pim Snel; push
