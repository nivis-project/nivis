# Tasks: refresh-destroy-cli

- [x] 1.1 `internal/graph/dag.go`: `DestroyOrder()` (reverse of `ApplyOrder`,
      deterministic). Also fixed Build to add ordering deps from `__derived`
      inputs (Nix-mediated deps are real for ordering, even though resolved by
      re-eval) via `resourceIDOf`.
- [x] 1.2 `internal/destroy/destroy.go`: `Run` over `DestroyOrder` — refuse
      preventDestroy (named error), else encode stored state as PriorState +
      ApplyResourceChange with null planned state, then `store.Delete`. `--target`
      filter; only destroys resources present in state.
- [x] 1.3 `internal/refresh/refresh.go`: `Run` over resources in state — encode
      stored attrs, `ReadResource`, decode, write back. No plan/apply.
- [x] 1.4 `internal/tfvalue`: `EncodeState` (flat attrs -> DynamicValue) and
      `NullState` (null planned state for destroy).
- [x] 1.5 `cmd/nixform/main.go`: cobra root + plan/apply/destroy/refresh and
      `state list|show|rm`; flags `--flake`, `--state`, `--attr`, `--target`.
      plan/apply drive the phase loop; destroy/refresh use the engines (phase-0
      graph for dependency structure); state ops use the store.
- [x] 1.6 Unit tests: ApplyOrder/DestroyOrder (reverse, deterministic); destroy
      reverse order + preventDestroy + target; refresh round-trips state via
      ReadResource. Real fake binaries via the manager.
- [x] 1.7 e2e `tests/e2e/lifecycle_test.go`: after the headline apply, refresh
      leaves state unchanged and destroy removes C, B, A in reverse dep order.
- [x] 1.8 `go build ./cmd/nixform` (CLI smoke-tested: plan/apply/state/destroy);
      `go test ./...` + `go vet ./...` pass; nix tests + IR conformance green.
- [x] 1.9 `openspec validate refresh-destroy-cli` passes; change-id in beans E3b.
      The headline-e2e destroy/refresh assertions are now satisfied (lifecycle e2e).
