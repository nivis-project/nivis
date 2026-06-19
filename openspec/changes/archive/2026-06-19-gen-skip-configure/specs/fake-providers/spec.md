# Spec delta: fake-providers

## ADDED Requirements

### Requirement: A fake provider that rejects an unconfigured Configure
There SHALL be an in-repo fake provider whose `ConfigureProvider` returns an error
diagnostic when configured with a null/empty config (mimicking a credential-
requiring real provider such as proxmox), while still serving `GetProviderSchema`
normally. It exists so the "schema fetch skips configure" behaviour is provable
hermetically (no network, no real credentials): the existing fakes' configure
always succeeds, so they cannot catch a configure-before-schema regression. The
fake SHALL be available on the `#fake-providers` PATH (or buildable in the e2e).

#### Scenario: configure fails but schema is still served
- GIVEN the configure-rejecting fake spawned
- WHEN it is configured with an all-null config
- THEN configure returns an error diagnostic
- AND GetProviderSchema still returns the provider's resource schema.
