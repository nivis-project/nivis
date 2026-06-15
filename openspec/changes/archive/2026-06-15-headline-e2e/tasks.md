# Tasks: headline-e2e

- [x] 1.1 Add `nixform.planCycle` to the flake (`nix/example/cycle.nix`): the
      cyclic variant where `A.label` derives from `C.value` and `C.label` from
      `A.value` — neither can ever become ready.
- [x] 1.2 `tests/e2e/headline_test.go` `TestHeadlineRoundTrip`: drives the real
      flake `nixform.plan` + real fake binaries through the phase driver; asserts
      exactly 3 apply phases + fixpoint, ledger has A.id/A.value/B.endpoint/C.*,
      and systemConfig consumer concrete from both providers == deterministic
      derivations (recordEndpoint, tokenValue, combined).
- [x] 1.3 `TestCycleRejected`: drives `nixform.planCycle`; asserts the driver
      returns an actionable error naming A and C (unresolvable / cycle).
- [x] 1.4 `TestTwoPhaseCapInsufficient`: caps at 2 phases on the real topology;
      asserts it fails to resolve (proves N>2 required).
- [x] 1.5 Tests skip cleanly if `nix` is absent / binaries can't build; build the
      provider binaries into repo `bin/` (gitignored) so the flake `source` paths
      resolve.
- [x] 1.6 `go test ./...` + `go vet ./...` pass; nix tests + IR conformance green.
      Deferred destroy/refresh assertions tracked in beans nixform2-xtgz (E3b).
- [x] 1.7 `openspec validate headline-e2e` passes; change-id recorded in beans
      epic E4b.
