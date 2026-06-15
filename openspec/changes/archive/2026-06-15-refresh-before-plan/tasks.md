# Tasks: refresh-before-plan

## 1. Spec
- [x] 1.1 Write proposal, tasks, executor spec delta (MODIFIED plan/apply: refresh prior state before planning; ADDED refresh opt-out)
- [x] 1.2 `openspec validate refresh-before-plan` passes

## 2. Implement
- [x] 2.1 `Driver.NoRefresh` option; a helper that returns prior state = provider Read (when refresh on) else stored
- [x] 2.2 `PlanReport`: use the refreshed prior; read-empty (deleted) => create; no stored => create (no read)
- [x] 2.3 `applyOne`: same — refresh prior before plan; deleted => create; persist refreshed state
- [x] 2.4 `cmd/nivis`: `--refresh` flag (default true) on plan + apply

## 3. Tests + close
- [x] 3.1 Unit (stub client): drift (read≠stored → plan uses read), deletion (read empty → create), --refresh=false (no read)
- [x] 3.2 Full gate (build, go test, nix, IR conformance); gofmt
- [x] 3.3 `openspec archive refresh-before-plan`; fold into executor spec
- [x] 3.4 Close beans-q3ku; complete epic ft9v; commit as Pim Snel; push
