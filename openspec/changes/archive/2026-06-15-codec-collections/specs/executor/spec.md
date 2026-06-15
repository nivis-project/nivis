# Spec delta: executor

## ADDED Requirements

### Requirement: Value codec encodes and decodes collections and objects
The value codec SHALL encode and decode `list`, `set`, `tuple`, `map`, and
`object` (nested) attribute types, in addition to the scalar string/number/bool
types, for both the tfprotov5 and tfprotov6 backends. Encoding maps a Go
`[]interface{}` to list/set/tuple values and a `map[string]interface{}` to
map/object values, recursing into element/attribute types; decoding is the
symmetric inverse.

#### Scenario: round-trip a list of strings
- GIVEN a `list(string)` attribute with value `["a","b"]`
- WHEN it is encoded to a DynamicValue and decoded back
- THEN the decoded Go value is `["a","b"]`.

#### Scenario: round-trip a map of strings
- GIVEN a `map(string)` attribute with value `{ env = "prod" }`
- WHEN encoded and decoded
- THEN the decoded value is `{ "env": "prod" }`.

#### Scenario: round-trip a nested object
- GIVEN an `object({ name = string, ports = list(number) })` value
  `{ name = "x", ports = [80, 443] }`
- WHEN encoded and decoded
- THEN the decoded value preserves the nested structure and element types.

#### Scenario: unknown collection attribute at plan
- GIVEN a computed `list(string)` attribute with no config value
- WHEN config is encoded for plan
- THEN that attribute is the tftypes unknown value (unchanged from scalar behavior).

#### Scenario: unsupported type errors clearly
- GIVEN an attribute of a type the codec does not support (e.g. DynamicPseudoType)
- WHEN encoding is attempted
- THEN it returns an error naming the unsupported type rather than panicking.
