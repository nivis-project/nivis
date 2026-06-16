# docs/TESTING.md: testing strategy & the headline e2e

Testing is part of "done." No OpenSpec change is complete without its tests.

## Layers

1. **Pure Nix functions, property tests.** `mkResource`, the reference system,
   `for_each`/`count` expansion, and `toIR` conformance to `docs/IR-CONTRACT.md`.
   Property: for arbitrary valid resource graphs, every IR leaf is a value, a
   well-formed `__ref`, or a `__derived`; ids are unique; every edge endpoint
   exists.
2. **Go, table-driven unit tests.** IR ingestion/validation, DAG construction,
   ref classification (TF→TF vs \*→Nix), TF→TF in-executor resolution, the
   `__ref`→tfprotov6-unknown mapping, state read/write/lock, fixpoint detection.
3. **Integration, against fake providers, no network.** Executor spawns a fake
   `tfprotov6` provider, completes the go-plugin handshake, drives
   `GetProviderSchema`/`PlanResourceChange`/`ApplyResourceChange`, and persists
   state. Proves the protocol client end-to-end offline.
4. **E2E, the full pipeline.** `.nix` → IR → phased plan/apply → state →
   refresh → destroy, culminating in the headline test below.

All provider-touching tests use **in-repo fake providers** so the suite is
hermetic and runs in CI without credentials or registry/network access.

## The fake providers (Epic 4a)

Two minimal Go binaries that speak `tfprotov6`. Each returns a static schema and
produces **computed (unknown-at-plan)** outputs at apply.

**`provider-alpha`**, resource `alpha_token`:
- inputs: `label` (string, optional)
- computed outputs (known only after apply): `id` (string), `value` (string,
  derived deterministically from `label` + a counter so tests are reproducible)

**`provider-beta`**, resource `beta_record`:
- inputs: `from` (string, required)
- computed output (known only after apply): `endpoint` (string, derived from
  `from`)

Determinism: outputs are a pure function of inputs (+ a per-process counter that
the test harness seeds), so assertions are exact. No clocks, no randomness, no
external calls.

## Headline e2e: milestone exit criterion (Epic 4b)

**What it must prove, in one test:** two providers, unknown values originating on
**both** sides, resolution requiring **≥3 phases**, and a Nix-side consumer
reading outputs from **both** providers (the round trip). The dependency graph
is acyclic; the phase count comes from each hop being **Nix-mediated**
(`__derived`), which is exactly what forces N>2.

### Topology (`tests/e2e/two-providers.nix`)

```
alpha_token.A           (alpha)  : no inputs
   └─ A.value  ─┐
                ▼  Nix: name = "rec-" + A.value          (__derived on A.value)
beta_record.B  (beta)   from = name
   └─ B.endpoint ─┐
                  ▼ Nix: final = B.endpoint + "::" + A.value   (__derived on B.endpoint, A.value)
alpha_token.C  (alpha)  label = final

# Nix-side consumer reading from BOTH providers (simulated NixOS option):
systemConfig = {
  recordEndpoint = B.endpoint;   # from beta
  tokenValue     = A.value;      # from alpha
  combined       = final;        # from both
}
```

- Unknowns originate on **both** sides: `A.value`/`C.*` from provider alpha and
  `B.endpoint` from provider beta.
- The chain A → (Nix name) → B → (Nix final) → C is acyclic but each arrow
  crosses the Nix boundary via `__derived`, so it cannot collapse into one pass.

### Required phase progression

- **Phase 0 eval:** `A.config` fully known; `name` is `__derived` on `A.value`
  (unknown); `B`, `final`, `C`, `systemConfig` all pending.
- **Phase 1 apply:** only `A` is ready → apply `A` → ledger gains `A.id`,
  `A.value`.
- **Phase 1→2 eval:** re-eval injects `A.value` → `name` resolves →
  `B.config.from` now known; `final` still pending on `B.endpoint`.
- **Phase 2 apply:** `B` ready → apply `B` → ledger gains `B.endpoint`.
- **Phase 2→3 eval:** re-eval injects `B.endpoint` → `final` resolves →
  `C.config.label` known; `systemConfig` fully resolves (both providers present).
- **Phase 3 apply:** `C` ready → apply `C`. No pending refs remain.
- **Phase 4 eval:** produces no new resolved value → **fixpoint → halt.**

### Assertions

- Total phases that performed an apply == 3 (and the loop halts at fixpoint, not
  by a hardcoded count).
- Attempting to resolve with a 2-phase cap leaves `final`/`C`/`systemConfig`
  unresolved → the engine reports them as pending (proves >2 phases is *required*,
  not incidental).
- Final outputs ledger contains `A.id`,`A.value`,`B.endpoint`,`C.*`.
- `systemConfig` evaluates to concrete values for `recordEndpoint`, `tokenValue`,
  and `combined`, each matching the deterministic provider outputs, proving
  **TF→Nix feedback from both providers**.
- A cycle variant (make `A.label` depend on `C.*`) is rejected at fixpoint with
  an actionable "unresolvable / cycle" error naming A and C (Epic 3.5.3).
- `destroy` removes C, B, A in reverse dependency order; `refresh` reconciles
  state via `ReadResource` without changing the plan.

### Why this is the right exit test

It exercises every load-bearing decision at once: the protocol client (real
`tfprotov6` handshake to two providers), TF→TF and \*→Nix ref handling, the
`__derived` mechanism, N-phase fixpoint resolution with N>2, and the round trip
that is the project's entire reason for existing, with unknowns genuinely
originating on both provider sides.
