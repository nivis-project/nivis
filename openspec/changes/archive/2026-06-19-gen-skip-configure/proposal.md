# Proposal: gen-skip-configure

## Why
`nivis gen` cannot extract a schema from any provider that validates credentials
at **Configure** time (proxmox, azurerm, google, …), because the plugin manager
always calls `ConfigureProvider` before the client is returned, and `gen`
configures with an all-null config before fetching the schema. But
`GetProviderSchema` does not require configuration (Terraform/OpenTofu fetch the
schema before any credentials exist), so codegen should skip Configure entirely.
Reported by the **nivis-registry** companion project (beans-jcpm), whose
extraction pipeline must currently mark such providers `schema_extractable:false`.
The bug is silent because credential-free providers and AWS (which tolerates an
all-null configure) work, which is why it slipped past 0.4.0/0.4.1.

## What changes
- **A configure-free path to a provider client, for codegen only.** Add
  `Manager.ClientForSchema(identity, path)`: identical to `Client` (spawn,
  handshake, dispense, version-negotiate, pool by identity) but it does **not**
  call `ConfigureProvider`. The shared spawn/dispense/negotiate body is factored
  into a private helper so `Client` and `ClientForSchema` do not duplicate it.
- **`cmd/nivis/gen.go` calls `ClientForSchema`** instead of `Client(..., map{})`.
  `gen.Fetch` (which only uses `ListResourceTypes` + `GetSchema`) is unchanged.
- **Everything else is unchanged.** `Client` still calls `Configure`, so
  plan/apply/refresh/destroy still configure the provider exactly as before. Only
  the codegen path skips it.

This is protocol-correct: `GetProviderSchema` is invoked before Configure by the
upstream CLI too.

## Decisions
- **Option 1 from the bean** (`ClientForSchema` method + a shared private helper),
  not a flag on `Client`. `gen.go` uses the concrete `*plugin.Manager`, so adding
  a method does not touch the `ProviderManager` interface the phase driver depends
  on. The two entry points stay distinct and obvious.

## Non-goals
- Changing `Configure` behaviour for plan/apply/refresh/destroy (it must stay).
- Passing provider config to codegen (codegen needs none).
- Registry-side changes; the registry re-pins `nivis` after 0.4.2 and re-adds the
  credential-requiring providers to its proof set (no registry code change).

## Impact
- `internal/plugin/manager.go`: factor the spawn body into a private helper;
  `Client` = helper + Configure; new `ClientForSchema` = helper only. Pooling and
  identity reuse preserved for both.
- `cmd/nivis/gen.go`: call `mgr.ClientForSchema(identity, providerPath)`.
- A **configure-rejecting fake provider** so the fix is provable hermetically: a
  fake whose `ConfigureProvider` returns an error on an all-null config but serves
  `GetProviderSchema` normally (the existing fakes' Configure always succeeds, so
  they cannot catch this bug). Exposed on the `#fake-providers` PATH.
- Tests: extend `tests/e2e/codegen_test.go` to assert (a) `Client(..)` on the
  configure-rejecting fake **fails** at Configure (guards the plan/apply path), and
  (b) the schema path **succeeds** and `gen.Fetch` returns the resource(s); keep
  the existing all-null-configure-OK test passing.
- CHANGELOG `[Unreleased]`; release as **0.4.2**.

Changelog: Fixed `nivis gen` configuring the provider before fetching its schema,
which broke codegen for credential-requiring providers (proxmox, azurerm,
google); schema extraction no longer calls ConfigureProvider. plan/apply still
configure as before.

Docs impact: none - an internal codegen/plugin fix; the `nivis gen` user-facing
behaviour (generate constructors from a provider) is unchanged, it just works for
more providers now. (The changelog entry is the user-visible record.)
