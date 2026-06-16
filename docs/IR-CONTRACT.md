# docs/IR-CONTRACT.md: the frozen IR

This is the contract between the Nix library (Epic 1), codegen (Epic 2), and the
executor (Epic 3 / 3.5). **It is an API.** Changing its shape requires an
OpenSpec change to this document first, then downstream updates. Version it.

`schemaVersion` is bumped on any breaking change.

## Top-level shape

```jsonc
{
  "schemaVersion": 1,
  "providers": {
    "<provider-id>": {                 // e.g. "alpha", "registry.opentofu.org/x/alpha"
      "source": "<source-or-path>",    // for PoC: filesystem path to the binary
      "config": { /* provider config, may contain refs */ }
    }
  },
  "resources": [
    {
      "id": "<provider>.<type>.<name>",       // stable identity, unique
      "provider": "<provider-id>",
      "type": "<resource-type>",               // e.g. "alpha_token"
      "name": "<name>",
      "config": { /* attribute tree; leaves may be values, refs, or unknowns */ },
      "meta": {
        "dependsOn": ["<id>", ...],            // explicit edges (additive to implicit)
        "lifecycle": { "preventDestroy": false, "ignoreChanges": ["<path>"] }
        // count/for_each are NOT here: expansion already happened in Nix (see below)
      }
    }
  ],
  "edges": [
    { "from": "<id>", "to": "<id>", "via": "<config-path-in-to>" }  // dependency graph
  ],
  "nixConsumers": [
    // values Nix computed FROM resource outputs, surfaced for the round trip.
    // On a given phase these may still be unknown; they become concrete once
    // their inputs are resolved and Nix is re-evaluated.
    { "id": "<consumer-id>", "value": { /* tree; leaves may be values/refs/unknowns */ } }
  ]
}
```

## Reference encoding (the core of the contract)

A not-yet-known cross-resource or cross-domain value is a **typed ref leaf**:

```jsonc
{ "__ref": { "resource": "<id>", "path": ["attr"] } }                 // scalar attr
{ "__ref": { "resource": "<id>", "path": ["net", 0, "ip"] } }         // nested + list index
{ "__ref": { "resource": "<id>", "path": ["tags", "env"] } }          // map key
```

- `path` is an ordered list of string keys / integer indices into the source
  resource's output object. This covers nested attributes, list/set indices, and
  map keys uniformly. (For sets, index is the post-apply stable ordering the
  provider returns.)
- A ref **inside an expanded `for_each`/`count` instance** is just a normal ref
  whose `resource` is the concrete expanded id (e.g. `alpha.alpha_token.web["a"]`
  → id `alpha.alpha_token.web__a`). There is no special "expansion ref" because
  expansion is already done (below).
- A ref whose target resource does not yet exist in state is **unresolved**, not
  an error, until fixpoint (Epic 3.5.3).

### Ref classification (drives phase behavior, DESIGN D3)
The executor classifies each ref:
- **TF→TF**: the ref appears in a `resources[].config`. Resolved in-executor when
  the target's output is known; does **not** require Nix re-eval.
- **\*→Nix**: the ref appears in a `nixConsumers[].value`, or a `resources[].config`
  leaf that Nix itself derived from another resource's output (Nix marks these,
  see "derived" below). Resolving these requires re-eval with the outputs ledger
  injected.

## Unknown values (toward the provider)

When the executor calls `PlanResourceChange` with inputs that are still refs, it
must present them to the provider as the protocol's **unknown value** sentinel,
not as the `__ref` JSON (providers don't understand our refs). The mapping
`{ "__ref": ... }` → tfprotov6 unknown is the executor's responsibility (Epic
3a.5). Mine Pulumi's bridge for how it encodes unknowns at plan/diff time.

## `for_each` / `count` expansion timing

**Expansion happens in Nix.** The IR contains concrete, already-multiplied
resources with deterministic ids (`<base>__<key>`). The executor never sees
`count`/`for_each`; it only sees resolved instances and edges between them. This
keeps the Go `ResourceNode` simple and the graph explicit.

## "Derived" Nix values

A config leaf that Nix computed from a resource output (e.g.
`"web-" + alpha.id`) cannot be a plain `__ref` (it's a transformation). Nix emits
such a leaf as **unknown-pending** until the inputs are available:

```jsonc
{ "__derived": { "inputs": ["<id>.attr", ...] } }   // value computed by Nix once inputs known
```

The executor treats `__derived` leaves as \*→Nix: it cannot compute them; it
records that the listed inputs are required, and once those outputs are in the
ledger, the **next Nix re-eval** produces the concrete value. This is the
mechanism that forces N>2 phases for chained Nix-mediated dependencies.

## Build outputs (`__build`)

A config leaf that is the **output of a Nix build** (e.g. a resource `source`
that is a built disk image) is a `__build` leaf carrying its store path:

```jsonc
{ "__build": { "path": "/nix/store/<hash>-<name>/<file>" } }   // a Nix build output
```

Unlike `__ref`/`__derived`, a `__build` leaf is a **known** value: its path is
fixed at evaluation. But `nivis` *evaluates* (it does not build), so before a
resource is applied the executor **realises** each `__build` path it references
(building the derivation if the store path is not yet valid) and substitutes the
concrete path into the provider config. This is done per resource as it becomes
ready, so a build whose derivation depends on an earlier resource's apply-time
output is realised in a later phase: the build participates in the phased
fixpoint. A `__build` leaf is not an edge and not unknown-pending; authors emit it
with the `drv` helper (`source = drv image`).

## Sensitive values across the boundary

Provider schema marks attributes `sensitive`. Sensitive outputs:
- **Must not** be written into the IR JSON emitted by `nix eval` (that output and
  the Nix store are world-readable). The Nix side emits a ref/placeholder only.
- Live only in the executor's outputs ledger, which is written with restricted
  permissions (0600) and is **not** a Nix store path.
- When a sensitive output must feed a later Nix re-eval, it is injected via a
  private channel (file path passed as `--argstr`, file mode 0600), never baked
  into a derivation. The re-eval may use it but must not re-emit it into a
  world-readable output.

This is a hard requirement; getting it wrong leaks secrets into the store.

## Outputs ledger (the phased-eval injection format)

The file the executor accumulates and injects on each re-eval:

```jsonc
{
  "phase": 2,
  "outputs": {
    "<resource-id>": { "<attr>": <value-or-{__sensitiveRef}>, ... }
  }
}
```

Nix reads this (path passed in via the flake `plan` argument) to resolve refs
and compute `__derived` values on the next phase. `__sensitiveRef` points at the
restricted-mode channel rather than embedding the secret.

## Validation

The contract is **machine-checkable**, not just prose. Two artifacts make it so:

- **`docs/ir-schema.json`**: the normative JSON Schema (Draft 2020-12) encoding
  the structural rules of everything above: top-level shape, the
  `__ref`/`__derived`/`__sensitiveRef` leaf encodings, and the *no `count`/
  `for_each` in the IR* rule. A leaf-marker object (`__ref` etc.) is dispatched
  to its exact subschema, so a malformed marker reports a precise, addressed
  error (e.g. `at resources/1/config/label/__ref: 'path' is a required
  property`) rather than a generic failure.
- **`tests/ir-conformance/`**: the executable conformance suite. `check.py`
  layers (1) JSON-Schema structural validation over (2) the *referential* rules
  JSON Schema cannot express: unique ids, every `provider` declared, every edge
  endpoint present, every `__ref`/`__sensitiveRef` target existing. Fixtures
  under `fixtures/valid` and `fixtures/invalid` lock both directions; each
  invalid fixture asserts the error *names the offending element*.

Both producer/consumer sides MUST conform to these artifacts:
- **Nix** (Epic 1.5 `toIR`): a property test that, for arbitrary valid resource
  graphs, `toIR` output passes `check.py validate`: every leaf is a value, a
  well-formed `__ref`, a `__derived`, or a `__sensitiveRef`; ids unique; every
  edge endpoint exists.
- **Go** (Epic 3a.1 `IngestIR`): rejects malformed IR with an actionable error
  naming the offending resource/path (Epic 4c), matching the failure classes in
  `tests/ir-conformance/fixtures/invalid`. `check.py` is the reference behavior
  the Go validator is tested against.

Run the suite: `python3 tests/ir-conformance/check.py test`.
