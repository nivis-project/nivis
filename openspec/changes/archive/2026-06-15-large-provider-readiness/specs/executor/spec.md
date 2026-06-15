# Spec delta: executor

## ADDED Requirements

### Requirement: Provider schema is fetched once per spawned provider
A provider backend SHALL fetch the full provider schema at most once per spawned
process and serve `GetSchema` and `ListResourceTypes` from that cached response,
so operations over many resource types do not issue O(resources) schema RPCs.

#### Scenario: many GetSchema calls, one RPC
- GIVEN a spawned provider with many resource types
- WHEN GetSchema is called for each type and ListResourceTypes is called
- THEN the provider's GetProviderSchema RPC is invoked at most once.

### Requirement: Provider logs do not flood output
The plugin manager SHALL configure spawned providers with a quiet logger so their
debug/trace logging is not written to the executor's stderr; errors and operation
results remain visible.

#### Scenario: a chatty provider does not flood stderr
- GIVEN a provider that emits trace-level logs during schema fetch
- WHEN it is spawned and queried
- THEN its trace/debug logs are suppressed from the executor's stderr.
