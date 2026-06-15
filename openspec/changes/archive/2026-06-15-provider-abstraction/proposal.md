# Proposal: provider-abstraction

## Why
The executor speaks tfprotov6 directly: plan/apply/destroy/refresh/gen all call a
`tfplugin6.ProviderClient` and pass v6 protobuf types. Real providers (AWS,
Hetzner, almost all of the ecosystem) speak tfprotov5, so the executor must
support both. Before adding a v5 backend, this change introduces a
**version-neutral `provider.Client` interface** and moves the v6 path behind it,
with no behavior change — so the v5 backend (next change) plugs in without
touching plan/apply/etc.

## What changes
- New `internal/provider` package: a `Client` interface with the operations the
  executor actually uses — `GetSchema`, `Plan`, `Apply`, `Read` — exchanging
  **normalized Go types** (a schema model, a config/state attr map, diagnostics),
  not v5/v6 protobuf. Plus a `Schema` model (reuse the gen type model where it
  fits) and a normalized `Diagnostic`.
- A v6 backend `internal/provider/v6` implementing `Client` over the existing
  `tfplugin6` stubs + `internal/tfvalue` codec (moved/adapted as needed).
- Refactor `internal/plan`, `internal/apply`, `internal/destroy`,
  `internal/refresh`, `internal/gen`, and `internal/phase` to depend on
  `provider.Client` instead of `tfplugin6.ProviderClient`. The plugin manager
  returns a `provider.Client` (v6 backend for now).
- No new behavior: every existing unit/e2e test passes unchanged (the refactor's
  safety net).

## Non-goals
- The v5 backend itself (next change, `tfprotov5-client`).
- Registry download (later change, `provider-registry`).
- Changing the IR, the phased loop, or any user-visible behavior.

## Impact
- New: `internal/provider` (interface + normalized types), `internal/provider/v6`.
- Changed: plan/apply/destroy/refresh/gen/phase/manager now use `provider.Client`.
  `internal/tfvalue` becomes a v6-backend detail.
- Sets up `tfprotov5-client` to add a v5 backend behind the same interface, and
  the manager to negotiate the version per provider.
