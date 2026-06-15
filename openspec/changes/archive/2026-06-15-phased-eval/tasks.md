# Tasks: phased-eval

- [x] 1.1 `internal/ledger/ledger.go`: `Ledger{Phase, Outputs}` in the contract
      format; `Load`/`Save` (0600, atomic write + chmod), `Append`, `Known`,
      `Has`, `ToGraphOutputs`.
- [x] 1.2 `internal/phase/evaluator.go`: `NixEvaluator` interface; `NixEval`
      shells `nix eval .#nixform.plan --apply 'p: p (fromJSON (readFile <0600 file>))'
      --json --impure`; `StubEvaluator` for hermetic loop tests.
- [x] 1.3 `internal/phase/driver.go`: `Driver.Run` loop — Eval -> IngestIR ->
      ResolveTFTF(ledger) -> apply fully-known not-yet-applied resources via
      plan+apply -> Append outputs -> repeat while progress && work remains.
- [x] 1.4 Fixpoint + stuck detection: halt when a phase applies nothing new; if
      resources remain, error names each stuck resource and the `<id>.<attr>`
      inputs it awaits.
- [x] 1.5 `*->Nix` feedback: covered by the chain — a derived value becomes
      concrete in a later phase's IR and the consuming resource applies; the
      nixConsumer values are concrete at fixpoint (asserted in the integration test).
- [x] 1.6 Unit tests (StubEvaluator + real fake binaries + real state): 3-phase
      chain in exactly 3 apply phases and correct order; 2-phase cap fails
      (proves N>2 required); stuck resource named; ledger 0600 + round-trip.
- [x] 1.7 Integration test (real `nix eval` + fake binaries): the flake example
      through the driver -> 3 apply phases, fixpoint, final ledger has A/B/C
      outputs, systemConfig consumer concrete from BOTH providers and equal to
      the deterministic derivation. Skips if nix/binaries unavailable.
- [x] 1.8 `go test ./...` + `go vet ./...` pass; nix tests + IR conformance green.
- [x] 1.9 `openspec validate phased-eval` passes; change-id recorded in beans
      epic E3.5.
