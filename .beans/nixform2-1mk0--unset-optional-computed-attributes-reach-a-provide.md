---
# nixform2-1mk0
title: Unset optional-computed attributes reach a provider as unknown, so its schema defaults never fire
status: todo
type: bug
created_at: 2026-09-14T21:54:22Z
updated_at: 2026-09-14T21:54:22Z
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
