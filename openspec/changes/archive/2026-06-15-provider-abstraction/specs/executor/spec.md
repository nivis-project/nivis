# Spec delta: executor

## ADDED Requirements

### Requirement: Version-neutral provider client interface
The executor SHALL access providers through a version-neutral `provider.Client`
interface exposing `GetSchema`, `Plan`, `Apply`, and `Read`, exchanging
normalized Go types (schema model, attribute maps, diagnostics) rather than
protocol-version-specific protobuf. Plan/apply/destroy/refresh/codegen SHALL
depend only on this interface.

#### Scenario: v6 backend satisfies the interface
- GIVEN the tfprotov6 backend
- WHEN the executor drives a fake v6 provider through GetSchema/Plan/Apply/Read
- THEN it works through the `provider.Client` interface with no protocol types leaking to callers.

#### Scenario: behavior is unchanged by the abstraction
- GIVEN the existing fake-provider e2e and unit suites
- WHEN they run against the refactored executor
- THEN they pass unchanged (the abstraction introduces no behavior change).

### Requirement: Manager returns a version-neutral client
The plugin manager SHALL return a `provider.Client` for a spawned provider,
selecting the protocol backend internally; callers SHALL NOT depend on the
negotiated protocol version.

#### Scenario: manager hands back a Client
- GIVEN a provider binary
- WHEN the manager spawns it
- THEN it returns a `provider.Client` the executor can use directly.
