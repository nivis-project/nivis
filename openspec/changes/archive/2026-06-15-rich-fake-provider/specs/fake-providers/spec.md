# Spec delta: fake-providers

## ADDED Requirements

### Requirement: A fake provider exercises collection and object types
There SHALL be a fake tfprotov6 provider whose resource has collection (list,
map) and nested-object attributes, both as inputs and as computed outputs, so the
value codec is exercised end-to-end through a real spawn / plan / apply. Outputs
SHALL be deterministic functions of the inputs and the seedable counter.

#### Scenario: collection/object computed attrs are unknown at plan
- GIVEN the `delta_thing` resource with `tags` and `ports` inputs set
- WHEN it is planned
- THEN its computed `endpoints` (list) and `meta` (object) are the tftypes
  unknown value, and `id` is unknown.

#### Scenario: collection/object values round-trip through apply
- GIVEN a planned `delta_thing` with known `tags`/`ports` inputs (counter 0)
- WHEN it is applied through the plugin manager
- THEN the decoded state contains a concrete `endpoints` list and a concrete
  `meta` object, equal to the provider's deterministic derivation, with no
  unknowns remaining.

#### Scenario: a map/list input is delivered to the provider
- GIVEN `tags = { env = "prod" }` and `ports = [80, 443]` in config
- WHEN the resource is applied
- THEN the provider receives those collection inputs (reflected in the
  deterministic outputs derived from them).
