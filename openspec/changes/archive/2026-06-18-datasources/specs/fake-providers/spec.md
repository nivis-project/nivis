# Spec delta: fake-providers

## ADDED Requirements

### Requirement: A fake provider serves a datasource
At least one in-repo fake provider SHALL declare a datasource type and implement
`ReadDataSource`, returning deterministic attributes computed from the request
config, so the datasource path (Nix declaration, IR, executor read, ledger,
dependent resource) is testable hermetically with no network and no credentials.
The datasource's returned attributes SHALL be deterministic given the same config
(per the existing hermetic-outputs requirement), so tests can assert exact values.

#### Scenario: the fake returns deterministic datasource attributes
- GIVEN the fake provider's datasource type read with a given config
- WHEN ReadDataSource is called twice with the same config
- THEN both calls return identical attributes.

#### Scenario: a datasource output feeds a resource end to end
- GIVEN an IR with the fake's datasource and a resource whose config references a datasource output
- WHEN the phased loop runs against the fake binary
- THEN the datasource is read, its output resolves the resource's ref, and the resource applies with the datasource-derived value.
