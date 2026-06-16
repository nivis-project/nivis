# Spec delta: executor

## ADDED Requirements

### Requirement: Actionable error for nested-block list-vs-single mistakes
When converting a decoded config value to a provider value, the executor SHALL
emit an actionable error, naming the fix, for the two common nested-block shape
mistakes, instead of a type-jargon message. The offending attribute name SHALL be
included (the codec already prefixes per-key errors).

- When the target type is a **list, set, or tuple** and the supplied value is an
  **attrset** (decoded as a map), the error SHALL state that the block is
  list-nested and instruct the user to wrap the attrset in a one-element list
  (`[ { ... } ]`), rather than reporting `expected array for tftypes.List[...],
  got map`.
- When the target type is a **single-nested object or map** and the supplied
  value is a **list**, the error SHALL state that the block takes a single attrset
  and instruct the user to pass `{ ... }` rather than a list.

Valid inputs SHALL be unaffected, and other scalar/type mismatches SHALL keep
their existing messages. This requirement is about the error text only; it
changes no successful-conversion behavior and does not alter the IR.

#### Scenario: list-nested block given a bare attrset
- GIVEN a resource config where a list-nested block (e.g. `disk_container`, typed `List[Object{...}]`) is written as a bare attrset `{ ... }`
- WHEN the executor codes the config to a provider value
- THEN it returns an error that names the attribute and instructs the user to wrap the value in a one-element list `[ { ... } ]`
- AND the error does not consist solely of `expected array for tftypes.List[...], got map`.

#### Scenario: single-nested block given a list
- GIVEN a resource config where a single-nested block (typed `Object{...}`) is written as a list `[ { ... } ]`
- WHEN the executor codes the config to a provider value
- THEN it returns an error that names the attribute and instructs the user to pass a single attrset `{ ... }`, not a list.

#### Scenario: valid nested blocks still code successfully
- GIVEN a config where every nested block uses the correct shape (a one-element list for a list-nested block, a single attrset for a single-nested block)
- WHEN the executor codes the config to a provider value
- THEN conversion succeeds with no error and produces the same value as before this change.
