# Spec delta: executor

## ADDED Requirements

### Requirement: tfprotov5 provider backend
The executor SHALL support tfprotov5 providers via a backend implementing the
version-neutral `provider.Client` interface, covering schema listing/fetch,
plan, apply, read, and destroy with a v5 value codec.

#### Scenario: drive a v5 provider through the neutral interface
- GIVEN a fake tfprotov5 provider
- WHEN the executor runs GetSchema/Plan/Apply/Read/Destroy via provider.Client
- THEN each works and computed outputs become known at apply, identical to v6.

### Requirement: Protocol negotiation per provider
The plugin manager SHALL offer both protocol 5 and protocol 6 plugin sets and
SHALL build the backend matching the version go-plugin negotiates with the spawned
provider. Callers SHALL NOT specify or depend on the protocol version.

#### Scenario: v5 provider negotiates v5
- GIVEN a provider binary that serves only protocol 5
- WHEN the manager spawns it
- THEN the handshake negotiates v5 and a v5-backed provider.Client is returned.

#### Scenario: v6 provider still negotiates v6
- GIVEN a provider that serves protocol 6 (the existing fakes)
- WHEN the manager spawns it
- THEN it negotiates v6 and behavior is unchanged.

### Requirement: v5 providers work end-to-end in the phased loop
A tfprotov5 provider SHALL participate in the phased-eval loop exactly as a v6
provider: its computed outputs feed the ledger and unlock dependent resources.

#### Scenario: a v5 resource resolves in the loop
- GIVEN a graph mixing a v5 provider's resource with a dependent resource
- WHEN the driver runs to fixpoint
- THEN the v5 resource applies and its outputs resolve the dependent.
