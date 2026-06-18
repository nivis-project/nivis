# Spec delta: e2e

## ADDED Requirements

### Requirement: Stack outputs resolve end to end against the fake providers
There SHALL be an end-to-end test, against the in-repo fake provider binaries,
that exercises declared stack outputs: a configuration declares outputs derived
from resource outputs (including a value composed across more than one resource
that resolves across phases), it is applied to a fixpoint, and the executor's
output resolution returns the concrete values. The test SHALL assert both the
resolved map (names to concrete values) and that a value composed from multiple
resources is fully concrete (no placeholders), proving outputs ride the same
phased resolution as the round trip.

#### Scenario: declared outputs are concrete after a full apply
- GIVEN a flake (or IR) applied against the fake providers that declares an output equal to one resource's attribute and another output composed from two resources' attributes
- WHEN it is applied to a fixpoint and outputs are resolved
- THEN both outputs are present with concrete values, and the composed output equals the value built from the two resources' resolved attributes.
