# Spec delta: executor

## ADDED Requirements

### Requirement: Providers are configured before plan/apply
The executor SHALL call the provider's configure RPC (ConfigureProvider for v6,
Configure for v5) once per spawned provider, before any plan/apply/read, passing
the provider's `config` from the IR encoded against the provider config schema.
Attributes absent from the IR config SHALL be sent as null so the provider can
apply its own defaults (e.g. the AWS SDK credential/region chain). Configure
diagnostics SHALL surface as an error.

#### Scenario: configure happens before plan
- GIVEN a provider that requires configuration before planning
- WHEN the manager returns a client and a plan is requested
- THEN Configure has been called once for that provider first.

#### Scenario: configure errors surface
- GIVEN a provider that returns an error diagnostic from configure
- WHEN configuration runs
- THEN the operation fails with an error containing the diagnostic.

### Requirement: Schema object types include nested blocks
The executor SHALL include nested blocks (`block_types`) when building the
tftypes object type for a provider config or resource schema, as attributes whose
type reflects the nesting mode — SINGLE/GROUP as an object, LIST as a
list(object), SET as a set(object), MAP as a map(object) — recursing into the
nested block's own attributes and blocks. Omitting them yields a non-conforming
value (e.g. AWS configure fails "an object with 35 attributes is required").

#### Scenario: provider config object includes its blocks
- GIVEN a provider whose config has flat attributes plus nested blocks
- WHEN the provider config object type is built
- THEN it includes an attribute per nested block of the correct
  collection-of-object type, so a value with those attributes (null) conforms and
  Configure succeeds.

### Requirement: Real AWS provider can be planned read-only
The executor SHALL be able to configure the real AWS provider (credentials and
region resolved from the environment) and plan a resource without required inputs
(`aws_s3_bucket`), producing a planned state with no error and creating no
resource.

#### Scenario: plan aws_s3_bucket against real AWS
- GIVEN the real AWS provider and valid credentials in the environment
- WHEN the provider is configured and `aws_s3_bucket` is planned with no inputs
- THEN a planned state is returned, no error occurs, and no resource is created.
