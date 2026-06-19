---
# nixform2-56tm
title: 'nivis gen: provider attribute named ''name'' collides with the constructor''s reserved ''name'' (1379/2414 real resources emit invalid Nix)'
status: completed
type: bug
priority: normal
created_at: 2026-06-19T14:32:24Z
updated_at: 2026-06-19T14:52:02Z
---

## Summary

`nivis gen` emits a constructor lambda that always starts with the reserved
`name` argument (the Nivis resource *instance* name). When the provider's schema
ALSO has an attribute literally named `name`, the generated lambda contains the
formal `name` twice, so the `.nix` file is **invalid and won't evaluate**:

```
error: duplicate formal function argument 'name'
```

This is widespread on real providers: of the 2414 constructors the nivis-registry
PoC generated with **nivis 0.4.2**, **1379 fail to evaluate** — all from this one
collision. Per provider: **azurerm 926**, **google 450** (every `*_iam_policy`
resource has a `name` attr), **Telmate/proxmox 3**. Found while wiring 0.4.2 into
the registry (`~/cNivis/registry`); follow-up to the now-fixed `nixform2-jcpm`.

## Reproduce

```
nivis gen --provider <google or azurerm binary> --identity google --out /tmp/g
nix eval --impure --expr 'import /tmp/g/google/google_compute_disk_iam_policy.nix'
# error: duplicate formal function argument 'name'
```

A minimal repro: any resource whose schema has a top-level attribute named
`name`. Example generated head (proxmox `proxmox_cloud_init_disk`):

```nix
{ nivis }:
{ name, id ? null, meta_data ? null, name ? null, network_config ? null, ... }:
#  ^reserved instance name              ^provider's own 'name' attr  → DUPLICATE
...
nivis.mkResource ({ provider = "proxmox"; type = "proxmox_cloud_init_disk"; inherit name config; } // overrides)
```

## Root cause (exact code path)

`internal/gen/emit.go` `Emit(provider, r)`:

- It writes the lambda head literally: `b.WriteString("{ name")` (~line 47), then
  loops `for _, a := range inputs { fmt.Fprintf(&b, ", %s ? null", a.Name) }`
  (~line 48-50). If an input attribute is named `name`, it is appended a second
  time → duplicate formal.
- The same `name` is reused with two DIFFERENT meanings: the lambda's reserved
  `name` is the Nivis instance name, threaded into `mkResource` via
  `inherit name config` (~line 97). But a provider attribute called `name` is a
  *config field* that must land inside `config`, not be conflated with the
  instance name.

So this is not only a syntax dedup — `name`-the-instance and `name`-the-provider-
attribute are semantically distinct and both must reach `mkResource` correctly.

## The fix (design decision — pick what fits the IR/lib best)

The generator needs a reserved-name strategy so the constructor's own parameters
(`name`, `overrides`, and `nivis`) never collide with provider attributes, while
the provider's `name` attribute still flows into `config`. Options:

1. **Rename the reserved instance parameter** to something a provider attribute
   can't be, e.g. `_name` / `__name` / `resourceName`, and keep provider attrs
   verbatim. Then `inherit`-style threading uses the renamed param; the provider
   `name` attr stays `name` in `config`. (Cleanest semantically; changes the
   public constructor signature — every generated ctor's instance arg renames,
   and the nivis lib / any hand-written callers must follow. Worth checking the
   IR-CONTRACT and `nix/lib` for how `name` is consumed.)

2. **Keep the instance param `name`; namespace clashing provider attrs.** Detect
   any input/block whose name is in the reserved set {`name`, `overrides`,
   `nivis`} and accept it under a safe alias in the lambda (e.g. `cfg_name`)
   while emitting it into `config` under its real key:
   `// (if cfg_name == null then {} else { name = cfg_name; })`, and for a
   required clash `{ name = _cfg_name; }`. The reserved `name` stays the instance
   name. (Smaller blast radius — signature only changes for resources that
   actually collide; the alias should be documented in the file header so the
   reference is clear.)

3. **Pass provider config as one attrset arg** (e.g. `args ? {}`) instead of one
   formal per attribute — sidesteps all collisions but is a bigger redesign of
   the constructor ergonomics; probably out of scope here.

Recommendation: option 2 (or 1 if the lib already isolates the instance name) —
minimal, keeps the nice per-attribute signature, and only special-cases the
collision. Whatever is chosen, the generated file must `nix eval` and the
provider `name` attribute must still appear in `config` for resources that have
one. Apply the same reserved-name guard to `overrides` and `nivis` for safety
(an attribute named `overrides` would corrupt the override-merge today).

## E2E test (the proof)

The existing `tests/e2e/codegen_test.go` fakes have no `name`/`overrides`
attribute, so they don't catch this. Add coverage:

1. **A fake (or extend one) with a `name` attribute.** e.g. extend
   `cmd/provider-*` or `internal/fakeprovider` so a resource declares an
   attribute literally named `name` (and ideally a second resource with an
   attribute named `overrides`), plus a normal computed `id`.
2. **Unit test** in `internal/gen` (`emit_test.go`): assert `Emit(...)` for a
   resource with a `name` attribute produces a lambda with `name` appearing
   exactly once as a formal, and that the provider `name` attribute is emitted
   into `config`.
3. **Hermetic e2e** (`tests/e2e/codegen_test.go`, same style as
   `TestCodegenAgainstFake`): gen the colliding fake, then `nix eval` the emitted
   constructor with the lib and a value for the `name` attribute, and assert:
   (a) it evaluates (no "duplicate formal" / no eval error), and
   (b) the resulting resource has BOTH the right Nivis instance id AND the
   provider `name` value in its `config`.

## Acceptance criteria

- [ ] A provider attribute named `name` no longer produces a duplicate formal;
      the generated `.nix` evaluates.
- [ ] The provider's `name` attribute is preserved in `config` (required or
      optional handled correctly); the Nivis instance name still threads to
      `mkResource`.
- [ ] `overrides` / `nivis` collisions are guarded too (or explicitly documented
      as out of scope with a reason).
- [ ] Unit + hermetic e2e cover the collision; `go test ./...` + `nix flake
      check` green; CHANGELOG `[Unreleased]` notes the fix.

## Cross-project impact

Unblocks ~1379 currently-invalid constructors (azurerm 926 / google 450 /
proxmox 3 in the registry's proof set alone). The registry renderer already
tolerates the duplicate cosmetically, but the generated `.nix` is the actual
product — these resources can't be `nivis apply`-ed until this lands. After the
fix ships in a nivis release, the registry re-pins and the affected resources'
docs/constructors become valid with no registry change.

Related: `nixform2-jcpm` (gen skip-configure, fixed in 0.4.2), `nixform2-p4uz`
(nested-block codegen).



## Resolution (2026-06-19, OpenSpec change gen-reserved-names archived as 2026-06-19-gen-reserved-names)

Fixed via the bean's recommended option 2 (alias the colliding attribute; keep the instance `name`). internal/gen/emit.go now has a reserved-name guard:

- reservedFormals = { name, overrides, nivis } (the constructor's own parameters). formalFor(attr) returns the attr name, or a cfg_<name> alias when reserved.
- Lambda head, required-presence throws, optional-merge, and block emission all use the (possibly aliased) FORMAL, while config emits each attribute under its REAL key. The reserved `name` formal stays the Nivis instance name (inherit name config -> mkResource builds id provider.type.name). A provider `name` attr lands in config as `name`; a provider `overrides` attr lands in config as `overrides` and no longer corrupts the // overrides merge.
- The generated file header documents each alias (real key -> formal) so it stays a readable reference.

Acceptance criteria, all met:
- A provider attribute named name no longer produces a duplicate formal; the generated .nix evaluates. (Proven live + e2e.)
- The provider name attribute is preserved in config (required and optional handled); the instance name still threads to mkResource. (e2e asserts id = epsilon.epsilon_named.myinstance AND config.name = providerName.)
- overrides / nivis collisions guarded too (overrides exercised in the unit test + e2e; nivis is in the reserved set).
- Unit (internal/gen/emit_test.go) + hermetic e2e (tests/e2e/codegen_test.go TestCodegenReservedNameCollision, against the new epsilon_named fake resource on provider-epsilon). go test ./... + nix tests + docs-ssot gate green; CHANGELOG [Unreleased] notes the fix.

Cross-project handoff to nivis-registry (~/cNivis/registry): ships in the next nivis release (0.4.3 candidate; the release is the separate user-driven step). Once tagged, re-pin the nivis input and re-generate: the ~1379 previously-invalid constructors (azurerm 926, google 450, proxmox 3 in the proof set) become valid Nix with no registry change. Related: nixform2-jcpm (gen skip-configure, 0.4.2), nixform2-p4uz (nested-block codegen).
