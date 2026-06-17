# Spec delta: nix-lib

## ADDED Requirements

### Requirement: Declare typed variables with mkVars
The Nix library SHALL provide `mkVars`, a helper to declare configuration
variables with optional type and default, and to resolve them against the values
the executor injects in `ledger.vars`. `mkVars` takes a declaration attrset
mapping each variable name to `{ type ? "any"; default ? <unset>; }` and the
injected values, and returns an attrset of resolved, validated values the config
reads (e.g. `vars.region`). Behavior:

- A declared variable that is **set** (present in the injected values) resolves to
  the injected value.
- A declared variable that is **unset** resolves to its `default` if it has one.
- A declared variable that is **unset and has no default** is **required**:
  `mkVars` SHALL throw an actionable error naming the variable.
- The supported types are at least `str`, `int`, `bool`, and `any` (no
  validation). A set value whose type does not match its declaration SHALL throw
  an actionable error naming the variable and the expected type.
- The library SHALL stay pure (builtins only); `mkVars` performs no IO and reads
  no environment. It validates data already passed in.

`mkVars` SHALL be exported from the library so a flake can write
`vars = nivis.mkVars { … } (ledger.vars or {})` and read `vars.<name>` in `plan`.

#### Scenario: a set variable resolves to its value
- GIVEN `mkVars { region = { type = "str"; default = "eu-central-1"; }; }` and injected `{ region = "us-east-1"; }`
- WHEN it resolves
- THEN `vars.region` is `"us-east-1"`.

#### Scenario: an unset variable falls back to its default
- GIVEN the same declaration and injected `{ }`
- WHEN it resolves
- THEN `vars.region` is `"eu-central-1"`.

#### Scenario: a required variable that is unset throws
- GIVEN `mkVars { suffix = { type = "str"; }; }` (no default) and injected `{ }`
- WHEN it resolves
- THEN evaluation throws an error naming `suffix` as a required variable.

#### Scenario: a wrong-typed value throws
- GIVEN `mkVars { count = { type = "int"; }; }` and injected `{ count = "three"; }`
- WHEN it resolves
- THEN evaluation throws an error naming `count` and the expected type `int`.

#### Scenario: an undeclared injected value is ignored
- GIVEN `mkVars { a = { type = "str"; default = "x"; }; }` and injected `{ a = "y"; b = "z"; }`
- WHEN it resolves
- THEN `vars.a` is `"y"` and the result has no `b` (only declared variables are returned).
