# Spec delta: executor

## MODIFIED Requirements

### Requirement: Version-neutral provider client interface
The executor SHALL access providers through a version-neutral `provider.Client`
interface exposing `GetSchema`, `Plan`, `Apply`, and `Read`, exchanging
normalized Go types (schema model, attribute maps, diagnostics) rather than
protocol-version-specific protobuf. The plan request SHALL carry an optional
prior state and the plan result SHALL carry whether replacement is required; the
apply request SHALL carry the prior state. Plan/apply/destroy/refresh/codegen
SHALL depend only on this interface. The normalized schema returned by
`GetSchema` SHALL include the resource's **nested blocks**: for each, a name, a
nesting mode (single, list, set, or map), and its inner attributes (recursively),
so callers (notably codegen) need not parse protocol-specific protobuf to learn a
resource's blocks. Both the v6 and v5 backends SHALL populate this from the
provider schema's block types.

#### Scenario: v6 backend satisfies the interface
- GIVEN a v6 provider
- WHEN accessed through the manager
- THEN it is usable via the version-neutral client for schema/plan/apply/read.

#### Scenario: the normalized schema surfaces nested blocks
- GIVEN a resource whose provider schema declares a list-nested block and a single-nested block
- WHEN its schema is fetched through the version-neutral client (v6 or v5)
- THEN the returned schema reports both blocks with their nesting modes and inner attributes.
