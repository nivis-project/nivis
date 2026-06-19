# Spec delta: executor

## MODIFIED Requirements

### Requirement: Providers are configured before plan/apply
The executor SHALL call the provider's configure RPC (ConfigureProvider for v6,
Configure for v5) once per spawned provider, before any plan/apply/read, passing
the provider's `config` from the IR encoded against the provider config schema.
Attributes absent from the IR config SHALL be sent as null so the provider can
apply its own defaults (e.g. the AWS SDK credential/region chain). Configure
diagnostics SHALL surface as an error.

**Schema fetching SHALL NOT configure the provider.** Codegen needs only
`GetProviderSchema` (via `ListResourceTypes`/`GetSchema`), which the protocol
allows before configuration, so the executor SHALL provide a configure-free way to
obtain a client for schema fetching (distinct from the plan/apply client, which
still configures). This SHALL NOT change the plan/apply/refresh/destroy path: those
SHALL still configure the provider. The configure-free path SHALL therefore work
against providers that reject an unconfigured/all-null configure (credential-
requiring providers such as proxmox, azurerm, google), whose schema is otherwise
unreachable to codegen.

#### Scenario: configure happens before plan
- GIVEN a provider that requires configuration before planning
- WHEN the manager returns a client and a plan is requested
- THEN Configure has been called once for that provider first.

#### Scenario: configure errors surface
- GIVEN a provider that returns an error diagnostic from configure
- WHEN configuration runs
- THEN the operation fails with an error containing the diagnostic.

#### Scenario: schema fetch skips configure
- GIVEN a provider whose configure rejects an all-null config (credential-requiring)
- WHEN a client is obtained via the configure-free schema path and its schema is fetched
- THEN Configure is not called and the schema is returned successfully.

#### Scenario: plan/apply still configures that same provider
- GIVEN the same configure-rejecting provider
- WHEN a plan/apply client is obtained (the normal path)
- THEN Configure is still called and the operation fails with the configure diagnostic (the plan/apply contract is unchanged).
