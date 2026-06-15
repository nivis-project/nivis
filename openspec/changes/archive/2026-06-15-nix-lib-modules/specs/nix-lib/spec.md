# Spec delta: nix-lib

## ADDED Requirements

### Requirement: count expansion in Nix
The library SHALL expand a `count = N` resource into N concrete instances with
deterministic ids `<base>__<index>` (index 0..N-1) during Nix evaluation; the IR
SHALL contain only the expanded instances and no `count` field.

#### Scenario: count produces concrete instances
- GIVEN a resource `web` with `count = 3`
- WHEN expanded and serialized
- THEN the IR contains `...web__0`, `...web__1`, `...web__2` and no `count`.

### Requirement: for_each expansion in Nix
The library SHALL expand a `forEach = { <key> = <value>; ... }` resource into one
concrete instance per key with id `<base>__<key>`, mapping via `builtins.mapAttrs`
so each instance's config may use its key/value. The IR SHALL contain only the
expanded instances and no `forEach` field.

#### Scenario: for_each produces keyed instances
- GIVEN a resource `tok` with `forEach = { a = "A"; b = "B"; }`
- WHEN expanded and serialized
- THEN the IR contains `...tok__a` and `...tok__b` and no `forEach`.

#### Scenario: ref into an expanded instance
- GIVEN another resource referencing the `a` instance's output
- WHEN serialized
- THEN the ref's `resource` is the concrete id `<base>__a` (an ordinary `__ref`).

### Requirement: Meta-arguments carried to the IR
`mkResource` SHALL carry `dependsOn` and `lifecycle` (preventDestroy,
ignoreChanges) into the IR `meta`, and these SHALL NOT include `count`/`forEach`
(expansion already happened).

#### Scenario: dependsOn and lifecycle serialize
- GIVEN a resource with `dependsOn = ["a.t.X"]` and `lifecycle.preventDestroy = true`
- WHEN serialized
- THEN the IR `meta` contains those and no count/forEach.

### Requirement: Module composition merges to one flat graph
The library SHALL provide an `evalModules`-style entry that takes a list of
modules (`{ config, tf, lib, ... }: { resources, providers, nixConsumers }`),
merges their resources/providers/consumers into a single flat graph, and lets a
module reference resources declared in other modules via `tf`. Conflicting
resource ids SHALL be an error naming the id.

#### Scenario: resources across modules merge
- GIVEN module M1 declaring resource A and module M2 declaring resource B
- WHEN the modules are evaluated and serialized
- THEN the IR contains both A and B in one resource list.

#### Scenario: a module reads another module's resource via tf
- GIVEN M2's config references `tf."<A-id>"` (declared in M1)
- WHEN evaluated and serialized
- THEN the reference is a `__ref`/`__derived` to A, resolvable like any other.

#### Scenario: duplicate id across modules is rejected
- GIVEN two modules both declaring a resource with the same id
- WHEN evaluated
- THEN evaluation fails with an error naming the duplicated id.
