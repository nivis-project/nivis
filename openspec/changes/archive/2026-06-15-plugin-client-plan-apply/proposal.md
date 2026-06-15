# Proposal: plugin-client-plan-apply

## Why
The executor core (change `executor-core`) ingests IR, builds the DAG, and
resolves TF→TF refs — but it cannot yet talk to a provider. This change adds the
provider plugin client and the plan/apply engines, completing the single-provider
round trip the phased-eval loop (E3.5) drives: spawn a provider binary, complete
the go-plugin v6 handshake, and run `GetProviderSchema` → `PlanResourceChange` →
`ApplyResourceChange`, persisting computed outputs to state.

This is the second of E3's two changes. It is the piece that exercises DESIGN D2
(spawn unmodified providers, speak the protocol) for real.

## What changes
- Vendor `proto/tfplugin6.proto` (verbatim from terraform-plugin-go; the upstream
  Go stubs are in an un-importable internal package) and generate the client
  stubs into `internal/tfplugin6` (`proto/generate.sh`). This is the sanctioned
  path the proto file itself documents.
- `internal/plugin`: a manager that spawns a provider binary and completes the
  go-plugin/gRPC **v6** handshake (matching tf6server's magic cookie and
  protocol version), returning a `tfplugin6.ProviderClient`. Connections are
  pooled by provider identity and closed on shutdown. No muxer (PoC).
- Plan engine: per ready resource, `GetProviderSchema` → encode config to a
  `DynamicValue` (mapping unresolved `__ref` leaves to the tfprotov6 **unknown**
  value) → `PlanResourceChange`; surface a human-readable diff. No side effects.
- Apply engine: `ApplyResourceChange`; read the computed outputs from the new
  state; write partial state after each success so a failed run is recoverable.
- An integration test that drives the **real** `provider-alpha`/`provider-beta`
  binaries through the manager (spawn + handshake + plan + apply), asserting the
  computed outputs match the fakes' deterministic derivations.

## Non-goals
- The phased re-eval loop / outputs ledger / fixpoint (E3.5) — this change makes
  one phase's plan+apply work against ready resources; the loop is the next epic.
- Nix evaluation / `toIR` (E1) — tests feed IR JSON fixtures directly.
- Refresh/destroy/CLI (E3b), schema codegen for real providers (E2),
  sensitive-value private channel (exercised in E3.5 where it is needed).
- `__derived` resolution — that is *→Nix (re-eval), out of scope here; the plan
  engine treats a resource with derived leaves as not-ready.

## Impact
- New: `proto/`, `internal/tfplugin6/` (generated), `internal/plugin/`,
  `internal/plan/`, `internal/apply/`. New direct deps: `hashicorp/go-plugin`,
  `google.golang.org/grpc`, `google.golang.org/protobuf`.
- Completes E3. The plan/apply engines + plugin manager are what E3.5's phase
  driver calls each phase; the `__ref`→unknown mapping established here is the
  contract's unknown-representation requirement made real.
