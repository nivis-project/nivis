# Spec delta: nix-lib

## ADDED Requirements

### Requirement: Datasource construction with mkData
The Nix library SHALL provide `mkData`, mirroring `mkResource`, to declare a
datasource: `mkData { provider; type; name; config; }` returns an attrset with a
stable id `data.<provider>.<type>.<name>`, the config, and a `refAttr` (and
nested-path ref) accessor exposing the datasource's outputs as referenceable Nix
values, so a resource (or another datasource) can wire a datasource output into
its config exactly as it references a resource. `toIR` SHALL serialize declared
datasources into the IR's `dataSources` array and SHALL treat a ref to a
datasource id like any other cross-node ref (producing an edge). `mkData` SHALL be
exported from the library.

#### Scenario: mkData yields a referenceable datasource
- GIVEN `d = mkData { provider = "x"; type = "x_ami"; name = "ubuntu"; config = { ... }; }`
- WHEN a resource sets `ami = d.refAttr "id"` and toIR runs
- THEN the IR `dataSources` array contains `d` with id `data.x.x_ami.ubuntu`, and the resource config carries a `__ref` to that id with a corresponding edge.

#### Scenario: a datasource config may itself reference another node
- GIVEN a datasource whose config reads another resource's output via refAttr
- WHEN toIR runs
- THEN the datasource config carries the `__ref` and an edge from the target to the datasource exists.
