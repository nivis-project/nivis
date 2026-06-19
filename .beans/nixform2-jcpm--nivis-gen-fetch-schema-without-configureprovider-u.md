---
# nixform2-jcpm
title: 'nivis gen: fetch schema without ConfigureProvider (unblocks credential-requiring providers)'
status: completed
type: task
priority: normal
created_at: 2026-06-19T12:55:09Z
updated_at: 2026-06-19T13:27:49Z
---

## Summary

`nivis gen` cannot extract a schema from any provider that validates credentials
at **Configure** time, because `gen` configures the provider (with an all-null
config) **before** fetching the schema. The schema RPC (`GetProviderSchema`) does
not require configuration per the Terraform plugin protocol, so `gen` should skip
Configure entirely. Fixing this unblocks the whole class of credential-requiring
providers (proxmox, azurerm, google, …) for codegen.

Reported by the **nivis-registry** companion project (`~/cNivis/registry`), whose
extraction pipeline calls `nivis gen` on real providers. Today it must skip those
providers and mark them `schema_extractable:false`. Once this ships in **0.4.2**,
the registry re-pins the `nivis` input and they extract with no registry change.

## Reproduce (verified 2026-06-19 against v0.4.0 and v0.4.1 — bug present in both)

```
nivis gen --provider <telmate/proxmox binary> --identity proxmox --out /tmp/out
# ERROR ... tf_rpc=Configure ... "your API TokenID username should contain a !, check your API credentials"

nivis gen --provider <hashicorp/azurerm binary> ...
# ERROR ... tf_rpc=Configure ... "could not configure AzureCli Authorizer: ... \"az\": executable file not found in $PATH"

nivis gen --provider <hashicorp/google binary> ...
# ERROR ... tf_rpc=Configure ... "could not find default credentials ..."
```

Credential-free providers (random/null/tls/time) work, and **AWS works** only
because it tolerates an all-null configure (falls back to the default credential
chain without erroring). So the bug is silent for the providers most likely to be
tested first, which is why it slipped past 0.4.0/0.4.1.

## Root cause (exact code path)

`cmd/nivis/gen.go` (the `gen` RunE, ~line 43) obtains the client via
`mgr.Client(identity, providerPath, map[string]interface{}{})`, and
`internal/plugin/manager.go` `func (m *Manager) Client(...)` **always** calls
`cl.Configure(...)` at the end (currently ~line 161-164):

```go
// internal/plugin/manager.go  (~161)
// Configure the provider once, before it is used for plan/apply. An empty
// config is a valid no-op for config-free providers (the fakes).
if err := cl.Configure(context.Background(), config); err != nil {
    c.Kill()
    return nil, fmt.Errorf("plugin %q: configure: %w", identity, err)
}
```

`gen` then calls `gen.Fetch(ctx, client)` (`internal/gen/schema.go`), which only
uses `client.ListResourceTypes` + `client.GetSchema` — **neither needs
Configure**. The Configure call is correct for plan/apply/refresh/destroy; it is
wrong for codegen.

## The fix

Give `gen` a configure-free path to a `provider.Client`. Two acceptable shapes
(implementer's choice — pick the one that fits the Manager design best):

1. **A `Manager` method that skips Configure**, e.g.
   `func (m *Manager) ClientForSchema(identity, path string) (provider.Client, error)`
   — identical to `Client` but omits the final `cl.Configure(...)`. `gen.go` calls
   this instead of `Client(..., map{})`. (Refactor the shared spawn/dispense/
   version-negotiation body into a private helper so the two entry points don't
   duplicate it.)
2. **An option on `Client`**, e.g. `Client(identity, path, config, plugin.SkipConfigure())`
   or a `configure bool` — if you prefer one entry point. Functionally the same:
   for codegen, do not call `Configure`.

Keep `Configure` exactly as-is for every other caller (plan/apply/refresh/destroy
must still configure). Only the `gen` codegen path changes. `gen.Fetch` itself
needs no change.

`GetProviderSchema` is invoked before Configure by Terraform/OpenTofu too (it is
how the CLI discovers the schema before any credentials exist), so this is
protocol-correct, not a hack.

## E2E test (the proof — this is the important part)

The existing `tests/e2e/codegen_test.go::TestCodegenAgainstFake` does NOT catch
this bug: the fakes' `ConfigureProvider` always succeeds
(`internal/fakeprovider/base.go:93` returns an empty response). We need a fake
that **fails an all-null Configure**, mimicking proxmox/azure, and a test that
asserts `gen` succeeds anyway.

Recommended steps:

1. **Add a configure-rejecting fake.** Either:
   - extend `internal/fakeprovider` so a `Resource`/`Server` can be marked
     "requires configuration" → its `ConfigureProvider` returns an ERROR
     diagnostic when the incoming config is null/empty (and succeeds when a
     required attr is set); OR
   - add a small dedicated fake (e.g. `cmd/provider-epsilon`) whose
     `ConfigureProvider` always returns an error diagnostic, and which still
     serves `GetProviderSchema` normally.
   Mirror the real failure: return a `*tfprotov6.Diagnostic{Severity: ERROR,
   Summary: "missing credentials"}` from `ConfigureProvider`.

2. **Hermetic e2e** (extend `tests/e2e/codegen_test.go`, same style as
   `TestCodegenAgainstFake` — `requireNix`, `buildBinaries`, `plugin.NewManager`,
   bare-name spawn via `$PATH`):
   - Assert `mgr.Client(identity, "provider-epsilon", map[string]interface{}{})`
     **fails** at Configure (locks in the reproduction / guards against regressing
     the plan/apply path).
   - Assert the new schema path (`ClientForSchema` or the skip-configure option)
     **succeeds**, and `gen.Fetch` returns the resource(s) with the expected
     types/attrs. This is the green proof that codegen no longer configures.
   - Optionally emit + `nix eval` the constructor (as the existing test does) to
     confirm the end-to-end gen output is still valid.

3. Keep `TestCodegenAgainstFake` (the all-null-configure-OK case) passing too, so
   both the "tolerates configure" and "rejects configure" providers generate.

## Acceptance criteria

- [ ] `nivis gen` fetches the schema WITHOUT calling `ConfigureProvider`.
- [ ] `Configure` is unchanged for plan/apply/refresh/destroy (still called there).
- [ ] A configure-rejecting fake provider exists and is on the `nix shell
      .#fake-providers` PATH (or buildable in the e2e).
- [ ] Hermetic e2e proves: `gen` succeeds on the configure-rejecting fake; the
      plan/apply client path still errors on it (Configure still enforced there).
- [ ] `go test ./...` and `nix flake check` green; CHANGELOG `[Unreleased]` notes
      the fix; release as **0.4.2**.

## Cross-project handoff

After 0.4.2 ships, the **nivis-registry** repo will: `nix flake update nivis` to
pin v0.4.2, add `Telmate/proxmox` (+ an azure/google smoke) back into the proof
set (`scripts/proof.sh` and `tools/e2e/e2e_test.go::TestRealProviderPipeline`),
and confirm the contract + rendered Nix docs + compat badge for them. No registry
code change is expected — only the pin and the proof-set additions.

Related: `nixform2-m83a` (C3: Provider registry integration), `nixform2-p4uz`
(nested-block codegen).



## Resolution (2026-06-19)

Fixed. nivis gen now fetches the schema via a configure-free client path (plugin.Manager.ClientForSchema), so GetProviderSchema is fetched WITHOUT calling ConfigureProvider. plan/apply/refresh/destroy still configure unchanged: Manager.Client keeps identical behaviour, factored over a shared private dispense helper (spawn/handshake/dispense/negotiate, no configure); Client = dispense + Configure, ClientForSchema = dispense only.

Proof is hermetic: new fake cmd/provider-epsilon rejects an all-null Configure but serves GetProviderSchema normally, and is on the nix shell .#fake-providers PATH. tests/e2e TestCodegenSkipsConfigure asserts both halves: Client(...) fails at configure on epsilon (guards plan/apply still enforce Configure), ClientForSchema(...) succeeds and gen.Fetch returns epsilon_thing. TestCodegenAgainstFake still passes (now via ClientForSchema).

OpenSpec change gen-skip-configure archived (executor + fake-providers specs updated). CHANGELOG [Unreleased] notes the fix. gofmt, go build ./..., go test ./..., nix tests, and docs-ssot gate all green.

Cross-project handoff to nivis-registry (~/cNivis/registry): ships in 0.4.2 (the release is the separate user-driven step). Once 0.4.2 is tagged: re-pin the nivis input, re-add proxmox/azurerm/google to the proof set, drop the schema_extractable:false skips. No registry code change needed.
