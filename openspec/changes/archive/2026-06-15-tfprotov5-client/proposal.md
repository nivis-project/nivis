# Proposal: tfprotov5-client

## Why
Real providers (AWS, Hetzner, almost the whole ecosystem) speak tfprotov5, but
the executor only spoke v6 (beans-1h7k). With the version-neutral
`provider.Client` seam in place (change `provider-abstraction`), this change adds
a **tfprotov5 backend** and makes the plugin manager **negotiate** the protocol
per provider, so the executor drives both v5 and v6 providers identically.

## What changes
- Vendor `proto/tfplugin5.proto` (verbatim from terraform-plugin-go; upstream Go
  stubs are internal) and generate client stubs into `internal/tfplugin5`
  (extend `proto/generate.sh`). Same sanctioned copy-and-generate path as v6.
- `internal/provider/v5`: a backend wrapping a `tfplugin5.ProviderClient` that
  implements `provider.Client` — GetSchema/Plan/Apply/Read/Destroy/ListTypes,
  with a v5 value codec (the v5 DynamicValue/Schema types; same tftypes).
- `internal/plugin`: offer both protocol plugin sets
  (`VersionedPlugins{5: ..., 6: ...}`) so go-plugin negotiates with the provider;
  after handshake, build the v5 or v6 backend based on the negotiated version.
- A **fake v5 provider** (`cmd/provider-gamma`) serving `tfprotov5` via
  `tf5server`, mirroring the alpha/beta fakes, so v5 is tested hermetically.
- Tests: the existing executor flows (plan/apply/destroy/refresh) driven against
  the fake v5 provider through the same `provider.Client`, asserting identical
  behavior to v6.

## Non-goals
- Registry download of real providers (next change, `provider-registry`).
- v5↔v6 muxing of a single provider (DESIGN parks the muxer) — we negotiate the
  one protocol a provider serves, not both at once.
- Exotic v5 schema features beyond what the executor needs (the same scalar set
  the v6 path supports for the PoC).

## Impact
- New: `proto/tfplugin5.proto`, `internal/tfplugin5` (generated),
  `internal/provider/v5`, `cmd/provider-gamma` (fake v5). Changed:
  `internal/plugin/manager.go` (negotiation), `proto/generate.sh`.
- After this, the executor speaks both protocols; the registry change can
  download real (v5) providers and they will actually run.
