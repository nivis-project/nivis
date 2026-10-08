---
# nixform2-1mk0
title: Unset optional-computed attributes reach a provider as unknown, so its schema defaults never fire
status: completed
type: bug
priority: normal
created_at: 2026-09-14T21:54:22Z
updated_at: 2026-10-08T22:40:20Z
---

A provider that declares a schema default for an optional-computed attribute
gets an empty value instead, on create. Terraform does not behave this way, so
any provider written against Terraform is affected.

## What happened

Applying `040_tunnel_target` in nivis-demos, the `nixos_activation` resource
failed after a three minute reachability timeout:

```
Activation failed: target poc-target-aws-01 did not become reachable
  within 3m0s: true: exit status 255
fish: Unknown command: connect
exec connect poc-target-aws-01 --relay ... --key ...
     ^~~~~~^
```

The ssh ProxyCommand is built as `"%s connect %s --relay %s --key %s"` with
`tunnel_command` first. That attribute is Optional+Computed with
`stringdefault.StaticString("nivis-tunnel")`. It arrived empty, so the command
began with a space and the shell tried to run `connect`.

`profile` on the same resource has the same shape, with a default of
`/nix/var/nix/profiles/system`. It would have been empty too, and
`nix-env --profile ""` fails later and less legibly.

## Cause

`internal/tfvalue/value.go`, `EncodeConfig`:

```go
case !present && computed[name]:
    // computed attributes the user didn't set become unknown
```

Terraform sends **null** in the config object for an attribute the
configuration does not set, whatever its computedness. Unknown belongs in the
PLAN, not in the config. terraform-plugin-framework applies a `Default` during
PlanResourceChange only where the config value is null; unknown is not null, so
the default never fires and the value stays unknown all the way to apply, where
`ValueString()` yields "".

The comment above `ProposedMsgPack` in the same file says EncodeConfig
"rightly" marks these unknown for a create. That is right for the proposed new
state and wrong for the config: the two objects have different contracts and
this conflates them.

## Why it is dangerous rather than merely wrong

It fails silently and late. Nothing reports a missing default; the provider
receives a plausible empty string and does something with it. Here it produced
a shell error three minutes into an apply. A provider that writes an empty
string into a path, a name or a policy would do something worse.

Schema defaults are ordinary in the ecosystem. Every provider using one is
affected, so the blast radius is not one resource.

## Related

nixform2-aur6: a replacement plans against the object it is about to destroy.
Same file, same area, same root shape: the objects Terraform passes to a
provider each have a distinct contract, and nivis does not yet keep them apart.

## Todo

- [ ] `EncodeConfig` sends null, not unknown, for attributes absent from config
- [ ] Confirm the planned new state returned by PlanResourceChange is what
      ApplyResourceChange receives, so a default applied at plan time survives
- [ ] Test: a resource whose optional-computed attribute has a schema default,
      unset in config, arrives at apply carrying that default
- [ ] nivis-demos then drops the explicit tunnel_command and profile in 040


## Blocked on nixform2-msl4

The fakes cannot see this bug and the fix breaks them, because they read
`Config` where a real provider reads `ProposedNewState` and cannot express a
schema default. Split agreed on 2026-10-09: fakes first (nixform2-msl4,
OpenSpec `fakes-model-the-plan-contract`), then this.

Two findings that shrink the work here once that lands:

- `EncodeConfig` never needs the `computed` map. Absent means null whatever
  the computedness, so the parameter drops out of the signature. The correct
  Config encoder does not know what is computed. `ValidateResourceConfig` and
  `ConfigureProvider` already call it with an empty map.
- Todo 2 is already satisfied: `PlannedState` is an opaque handle
  (`internal/plan/plan.go:35`) passed from plan to apply without re-encoding,
  so a default applied at plan survives.

The spec never asked for the broken behavior. `executor/spec.md:155` specifies
"resolved values known; unresolved refs unknown" and says nothing about unset
computed attributes. The `!present && computed[name]` branch dates from
`2e8f547` (the original E3 plugin-client commit) and no test passes a
non-empty `computed` map to `EncodeConfig`, so it is unspecified and
untested. The "rightly" comment on `ProposedMsgPack` was written later and
assumed the create path was correct; it is not evidence.


## OpenSpec

`encode-config-sends-null` (spec-driven; proposal, design, spec delta on
`executor`, tasks). Validated, not yet implemented. Unblocked: nixform2-msl4
shipped as commit 5ed8bf6.


## Summary of Changes

`EncodeConfig` now sends null for an attribute absent from config, whatever its
computedness, and the `computed` parameter is gone from its signature. Unknown
in a `Config` object once again means only what it should: the configuration
sets the attribute to something not resolved yet.

`schemaRaw.computed` stays, because `ProposedMsgPack` needs it for the update
path. That code was correct and is untouched.

### The v5 twin

The proposal assumed the v5 backend called `tfvalue.EncodeConfig`. It does not:
`internal/provider/v5/tfvalue5.go` carried its own `encodeConfig` with the
same branch, copied. Both are fixed. Fixing only the v6 path would have left v5
providers broken while the bean read closed.

### The split proved itself

Before inverting it, `TestDeltaDefaultBlockedByEncoderUnknown` failed with the
message written for it in nixform2-msl4: "EncodeConfig now sends null and the
default fires. nixform2-1mk0 is fixed, so invert this test". The substrate
detected the fix rather than being told about it. That is what the two-change
split was for.

### Verification

`go test ./...` green including `tests/e2e` (112s). The falsification checks
the tasks demanded all held: `internal/plugin/integration_test.go`,
`v5_test.go` and `tests/` are unedited, and the plan still reports
`[endpoints id meta]` unknown, so the default rule did not leak into
computed-only attributes. Both protocol versions covered.

`internal/tfvalue` gained eight tests that pass a config exercising computed
attributes, which no test did before. They assert the decoded msgpack rather
than the Go call. That absence is how the defect survived from commit
`2e8f547`.

### Remaining, and not ours

The bean's last todo (nivis-demos dropping the explicit `tunnel_command` and
`profile` in stack 040) belongs to the nivis-demos repository. It is now
unblocked there but is not part of this change.

OpenSpec change: `encode-config-sends-null`.
