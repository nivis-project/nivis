# Proposal: gen-reserved-names

## Why
`nivis gen` emits a constructor lambda that always begins with the reserved `name`
formal (the Nivis *instance* name, threaded into `mkResource` to build the id
`provider.type.name`). When a provider's schema also has an attribute literally
named `name`, the emitter appends a second `name ? null` formal, so the file is
invalid Nix:

```
error: duplicate formal function argument 'name'
```

This is widespread on real providers: of the 2414 constructors the nivis-registry
PoC generated with nivis 0.4.2, **1379 fail to evaluate** from this one collision
(azurerm 926, google 450 because every `*_iam_policy` has a `name` attr, proxmox
3). The same hazard exists for `overrides` (an attribute named `overrides` would
corrupt the override-merge) and `nivis` (the lib argument). Reported by the
nivis-registry companion project (`~/cNivis/registry`); follow-up to the now-fixed
nixform2-jcpm (`gen-skip-configure`, shipped in 0.4.2).

The collision is not only a syntax dedup: the instance `name` and a provider
attribute called `name` are semantically distinct. The instance name must keep
threading to `mkResource`; the provider's `name` is a config field that must land
inside `config` under its real key.

## What changes
- The emitter (`internal/gen/emit.go`) gains a **reserved-name guard**. The
  reserved set is `{ name, overrides, nivis }` (the constructor's own
  parameters). For any input attribute or block whose name is in the reserved
  set, the emitter:
  - accepts it in the lambda under a safe alias (e.g. `cfg_name`), not its real
    name, so no formal is duplicated and no constructor parameter is shadowed;
  - emits it into `config` under its **real key** (`name = …`), so the provider
    attribute is preserved exactly;
  - keeps the reserved `name` formal as the Nivis instance name, unchanged.
- Required vs optional clashing attributes are both handled (required: the
  presence-throw uses the alias; optional: the `// (if alias == null …)` merge
  uses the alias but writes the real key).
- The aliasing is documented in the generated file header so the constructor
  remains a readable per-provider reference.
- This keeps the per-attribute constructor signature (only colliding resources
  get an aliased formal) and does not change the instance-name parameter or the
  nivis library; the smallest blast radius (the bean's recommended option 2).

## Non-goals
- Renaming the reserved instance parameter across all generated constructors
  (the bean's option 1) or passing provider config as one attrset arg (option 3).
- Changing the IR or `mkResource`'s signature.

## Impact
- `internal/gen/emit.go`: the reserved-name guard in `Emit`.
- Tests: a unit test (`internal/gen/emit_test.go`) that a resource with a `name`
  (and an `overrides`) attribute emits each reserved formal exactly once and emits
  the provider attribute into `config` under its real key; a fake provider with a
  colliding `name` attribute and a hermetic e2e (`tests/e2e/codegen_test.go`) that
  the emitted constructor `nix eval`s and the produced resource carries both the
  Nivis instance id and the provider `name` value in its config.

Changelog: Fixed `nivis gen` emitting a duplicate `name` formal when a provider
attribute is named `name` (also `overrides`/`nivis`), which made the generated
`.nix` invalid; the colliding attribute is now aliased in the lambda and emitted
into `config` under its real key, while the Nivis instance name still threads to
mkResource.

Docs impact: none - an internal codegen fix; the `nivis gen` user-facing behaviour
(a constructor per resource type) is unchanged, the generated files just become
valid for the affected resources. The generated file header documents the alias.
