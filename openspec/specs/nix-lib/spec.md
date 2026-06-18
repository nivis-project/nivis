# Spec: nix-lib

## Purpose
The Nivis Nix library is the configuration frontend: users describe resources
as Nix values and the library serializes them to the canonical JSON IR the Go
executor consumes. It is the input side of the round trip — how a Nix author
references apply-time provider outputs (`__ref`) and computes values from them
(`__derived`), and how the flake `plan` interface re-resolves those against the
outputs ledger on each phase so the phased-eval loop (E3.5) converges. Output
conforms to `docs/ir-schema.json`, the same artifact the executor validates.
## Requirements
### Requirement: Resource construction with stable identity
`mkResource` SHALL accept `{ provider, type, name, config }` and return an
attrset with a stable id `<provider>.<type>.<name>` and a way to reference the
resource's (apply-time) output attributes as Nix values.

#### Scenario: id is derived from coordinates
- WHEN `mkResource { provider = "alpha"; type = "alpha_token"; name = "A"; config = {}; }` is evaluated
- THEN its id is `"alpha.alpha_token.A"`.

#### Scenario: output attribute access yields a ref
- GIVEN a resource A built with `mkResource`
- WHEN `A.value` (an output attribute) is accessed at eval time
- THEN it evaluates to a typed reference placeholder, not an error.

### Requirement: Reference encoding
Accessing an output attribute SHALL produce a `__ref` leaf
`{ "__ref": { "resource": <id>, "path": [<attr>...] } }`; a value computed *from*
an output (e.g. string interpolation) SHALL produce a `__derived` leaf
`{ "__derived": { "inputs": [<id.attr>...] } }`.

#### Scenario: direct reference is a __ref
- GIVEN B.config.from = A.value
- WHEN serialized at phase 0 (A unresolved)
- THEN the leaf is `{ "__ref": { "resource": "alpha.alpha_token.A", "path": ["value"] } }`.

#### Scenario: computed value is a __derived
- GIVEN name = "rec-" + A.value
- WHEN serialized at phase 0
- THEN the leaf is `{ "__derived": { "inputs": ["alpha.alpha_token.A.value"] } }`.

### Requirement: toIR produces contract-conforming IR
`toIR` SHALL serialize providers, resources, edges, and nixConsumers to the
canonical IR with `schemaVersion: 1`; refs and derived leaves encoded as above;
edges derived from `__ref` usage. Each provider's `config` SHALL be resolved
against the outputs ledger and encoded with the **same** two passes as resource
config, so a `__ref`/`__derived` in provider config resolves each phase and
encodes to wire shape (and plain values pass through unchanged); `source` passes
through verbatim. The output SHALL conform to `docs/ir-schema.json` and the
referential rules.

#### Scenario: output validates against the schema
- GIVEN a resource set with one ref between two resources
- WHEN `toIR` is evaluated and the JSON is checked by `tests/ir-conformance/check.py`
- THEN it validates (structural + referential) with no error.

#### Scenario: edges reflect references
- GIVEN B.config.from references A
- WHEN `toIR` runs
- THEN the IR contains an edge `{ from: A, to: B, via: "from" }`.

#### Scenario: provider config resolves against the ledger
- GIVEN a provider whose `config` contains a `__derived` value built from a resource output, and a ledger holding that output
- WHEN `toIR` runs with that ledger
- THEN the provider's `config` in the IR holds the concrete derived value, not the placeholder.

### Requirement: Plan interface accepts the outputs ledger
The flake `nivis.plan` SHALL be a function of an injected outputs ledger
(`{ phase, outputs }`, empty on phase 0) and SHALL resolve `__ref`/`__derived`
leaves whose inputs are present in the ledger to concrete values, leaving the
rest as placeholders.

#### Scenario: phase 0 leaves refs unresolved
- WHEN `plan` is evaluated with an empty ledger
- THEN `__ref`/`__derived` leaves depending on unknown outputs remain placeholders.

#### Scenario: ledger resolves a derived value
- GIVEN name = "rec-" + A.value and a ledger with `A.value = "v0"`
- WHEN `plan` is re-evaluated with that ledger injected
- THEN `name` resolves to the concrete string `"rec-v0"` in the IR (no longer a `__derived` leaf).

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

### Requirement: mkProvider constructs a validated provider declaration
`mkProvider` SHALL accept `{ source, config ? {} }` and return the IR provider
shape `{ source, config }`, raising a named error if `source` is absent or not a
string. Nested provider blocks (e.g. `default_tags`, `assume_role`, `endpoints`)
SHALL be expressed as ordinary nested attrsets/lists inside `config`; `toIR`
serializes them and the executor's object-type construction maps them to the
provider schema. `mkProvider` is the ergonomic front door; `toIR` SHALL continue
to accept a bare `{ source, config }` attrset for backward compatibility.

#### Scenario: a provider config flows from Nix to the IR
- GIVEN `providers.aws = mkProvider { source = "registry.opentofu.org/hashicorp/aws"; config = { region = "eu-central-1"; }; }`
- WHEN `toIR` is evaluated
- THEN `providers.aws.source` is the given source and `providers.aws.config.region` is `"eu-central-1"`.

#### Scenario: missing source is a named error
- WHEN `mkProvider { config = {}; }` is evaluated (no `source`)
- THEN evaluation fails with an error naming `mkProvider` and the missing `source`.

### Requirement: Flake apps build and run the CLIs
The flake SHALL expose `packages.<system>.{tn,tn-gen}` (with `default = tn`) that
build the `tn` and `tn-gen` binaries from source via nixpkgs `buildGoModule`
(Go toolchain from the pinned `nixpkgs` input; module deps pinned by a committed
`vendorHash`, no `vendor/` directory), and matching `apps.<system>.{tn,tn-gen}`
(with `default = tn`) so `nix run .#tn -- …` and `nix run .#tn-gen -- …` work.
System enumeration SHALL use a small inline helper, not `flake-utils`. Adding
these outputs SHALL NOT make the library outputs depend on nixpkgs: `lib` and
`nivis.*` SHALL still evaluate without forcing the nixpkgs input.

#### Scenario: nix run drives the CLI
- WHEN `nix run .#tn -- --version` is executed
- THEN it builds `tn` from source and prints the version.

#### Scenario: the library still evaluates input-free
- WHEN `nix eval .#lib` and `nix eval .#nivis.plan --apply 'p: p { phase = 0; outputs = {}; }'` are evaluated
- THEN they succeed without building or importing anything from the nixpkgs input
  (the library remains pure-builtins).

### Requirement: nivis.drv marks a build-output value
The library SHALL provide `drv` such that `drv <derivation>` (optionally
`drv <derivation> { file = "<relative-path>"; }`) returns a typed **`__build`
leaf** carrying the absolute store path of the derivation's output (the inner
file when `file`/`passthru.filePath` applies). The leaf serializes to
`{ "__build": { "path": "<store-path>" } }`. Unlike `__ref`/`__derived`, a
`__build` leaf is a **known** value (its path exists at evaluation) — it SHALL
pass through `resolve` unchanged and is realised by the executor before apply, not
resolved against the outputs ledger. It exists so an author marks "this value is a
build output that must be realised" explicitly, rather than relying on string
heuristics.

#### Scenario: drv yields a __build leaf
- WHEN `drv image` is evaluated for a derivation `image`
- THEN it returns `{ __build = { path = "<image store path>"; }; }`, and `toIR`
  emits that leaf verbatim.

#### Scenario: a __build leaf survives a phase unresolved-but-known
- GIVEN a resource config containing a `__build` leaf and a ledger missing other inputs
- WHEN `toIR` resolves the config against that ledger
- THEN the `__build` leaf is unchanged (it is not ledger-dependent), ready for the executor to realise.

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
- **String values are coerced to the declared scalar type.** Because the CLI
  (`--var`) and environment (`NIVIS_VAR_*`) always supply strings, an `int` or
  `bool` variable given a string value SHALL parse it to that type (`"5"` -> `5`,
  `"-3"` -> `-3`, `"true"`/`"false"` -> the boolean); a value already of the
  declared type (e.g. a typed `--var-file` JSON value) passes through; a string
  that does not parse to the type SHALL throw the named, typed error. `str` and
  `any` keep the value unchanged.
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

#### Scenario: a string int from the CLI is coerced
- GIVEN `mkVars { replicas = { type = "int"; }; }` and injected `{ replicas = "5"; }` (as `--var replicas=5` supplies)
- WHEN it resolves
- THEN `vars.replicas` is the integer `5`.

#### Scenario: a string bool from the CLI is coerced
- GIVEN `mkVars { on = { type = "bool"; }; }` and injected `{ on = "true"; }`
- WHEN it resolves
- THEN `vars.on` is the boolean `true`.

#### Scenario: an undeclared injected value is ignored
- GIVEN `mkVars { a = { type = "str"; default = "x"; }; }` and injected `{ a = "y"; b = "z"; }`
- WHEN it resolves
- THEN `vars.a` is `"y"` and the result has no `b` (only declared variables are returned).

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

### Requirement: Declare stack outputs with the outputs argument
`toIR` SHALL accept an optional `outputs` argument, an attrset mapping a name to a
value expression (which may contain refs/derived leaves over resource or
datasource outputs). Each named output SHALL be emitted as a reserved nixConsumer
with id `output.<name>` and value `{ value = <expr>; }`, so it is resolved by the
existing consumer machinery (fully concrete at fixpoint) without a new node type.
The `outputs` argument is optional; omitting it produces no output consumers.

#### Scenario: a declared output becomes a reserved consumer
- GIVEN `toIR { ...; outputs = { ip = r.refAttr "addr"; }; }`
- WHEN the IR is built
- THEN it contains a nixConsumer with id `output.ip` whose value carries the ref to `r.addr`.

#### Scenario: outputs may derive from multiple resources
- GIVEN an output whose value is a Nix-built string over two resources' outputs
- WHEN the IR is built and resolved against a ledger with both outputs
- THEN the `output.<name>` consumer resolves to the concrete computed value.

#### Scenario: outputs are optional
- GIVEN `toIR` called without `outputs`
- WHEN the IR is built
- THEN no `output.` consumers are present and the IR is otherwise unchanged.

