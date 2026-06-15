# Proposal: fake-providers

## Why
Every integration and e2e test in nixform runs against in-repo fake providers,
never the registry (network is restricted; DESIGN D6). They are the hermetic test
substrate the executor (E3), the phased loop (E3.5), and the headline e2e (E4b)
are all built on, so they are built early — right after the IR contract froze.

Two providers are required because the milestone exit criterion needs unknown
values originating on **both** provider sides. They must produce
**computed-unknown-at-plan** outputs that become **known, deterministic** values
at apply, so the phased-eval loop has real apply-time values to feed back into Nix.

## What changes
- Add a shared `fakeprovider` Go package that speaks `tfprotov6` over go-plugin:
  schema encoding, `DynamicValue` <-> `tftypes` marshalling, the plan/apply
  computed-unknown semantics, and unimplemented defaults for the RPCs a fake
  does not need. Per-provider code supplies only resource logic.
- Add `provider-alpha` (resource `alpha_token`) and `provider-beta` (resource
  `beta_record`) binaries, per the spec in `docs/TESTING.md`.
- Determinism: outputs are a pure function of inputs plus a per-process counter
  the harness can seed (env var), so test assertions are exact. No clocks,
  randomness, or external calls.

## Non-goals
- The executor / plugin client (E3) — it is the consumer of these binaries and
  is a separate epic. This change only proves the providers serve correctly via
  a direct in-process protocol test.
- Real provider download / registry (network-gated; separate bean).
- Data sources, functions, ephemeral resources, import, move — fakes return
  unimplemented for these; only the resource RPCs needed for plan/apply/read are
  meaningful.
- Provider-side sensitive attributes — none of the fake outputs are sensitive in
  this change; the sensitive-value path is exercised later where needed.

## Impact
- New: `internal/fakeprovider/` (shared), `cmd/provider-alpha/`,
  `cmd/provider-beta/`. New Go dependency `terraform-plugin-go` (fetched via the
  module proxy; confirmed reachable).
- Establishes the resource semantics the executor's plan/apply engine (E3a.5/6)
  and the headline topology (E4b) depend on. Output derivations are fixed here
  and consumed verbatim by the e2e assertions.
