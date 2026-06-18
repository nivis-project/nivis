# Spec delta: ir

## ADDED Requirements

### Requirement: Datasource nodes in the IR
The IR SHALL support a top-level `dataSources` array, distinct from `resources`,
for infrastructure the configuration **reads** rather than creates (the array is
optional and MAY be omitted when there are none). A datasource node SHALL have the
shape `{ "id", "provider", "type", "name", "config" }`, where:

- `id` is `data.<provider>.<type>.<name>`, unique across all datasource ids (and
  not colliding with a resource id);
- `provider` names a declared provider;
- `config` is an attribute tree whose leaves may be values, `__ref`, or
  `__derived`, exactly like a resource config.

A datasource node SHALL NOT carry `meta`/lifecycle: a datasource is read, never
planned, applied, or destroyed. A `__ref`/`__derived`/edge MAY target a datasource
id, and a datasource config MAY reference a resource or another datasource, so
datasources participate in the dependency graph and the phased fixpoint like any
other node. `dataSources` is OPTIONAL: an IR with none MAY omit it.

#### Scenario: a datasource node is carried distinctly from resources
- GIVEN a config declaring a datasource and a resource that references it
- WHEN toIR serializes the graph
- THEN the IR has a `dataSources` array containing `{ id: "data.<p>.<t>.<n>", provider, type, name, config }` and the resource's config carries a `__ref` to the datasource id.

#### Scenario: a datasource id is unique and namespaced
- GIVEN two datasources and a resource
- WHEN the IR is built
- THEN each datasource id begins `data.` and no two node ids (resource or datasource) collide.

#### Scenario: datasources are optional
- GIVEN a config with no datasources
- WHEN the IR is built
- THEN `dataSources` MAY be absent and the IR remains valid.
