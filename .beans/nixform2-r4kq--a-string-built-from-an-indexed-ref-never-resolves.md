---
# nixform2-r4kq
title: 'A string built from an indexed ref never resolves: ledgerLookup reads the index as the attribute name'
status: todo
type: bug
priority: high
tags:
    - discovered
created_at: 2026-10-16T00:00:00Z
updated_at: 2026-10-16T00:00:00Z
---

Found 2026-10-16 in the first real use of `nivis import` (nixform2-3utj),
migrating a Route53 zone and an SES identity from Terraform into nivis 0.10.0.
Not an import gap: it bites any author who interpolates one element of a
provider's list output.

## What happens

A plain indexed reference works. The executor decodes numeric path elements:

```go
// internal/graph/resolve.go
func index(v interface{}, key interface{}) (interface{}, bool) {
    case []interface{}:
        i, ok := asIndex(key)   // float64 | int | numeric string
```

A string built from one does not. `nivis.str` produces a `__derived` leaf, and
`__render` is a Nix function that `toIR` strips at serialization, so only Nix
can render it. Nix then fails to resolve the input:

```nix
# nix/lib/ref.nix
ledgerLookup = ledger: key:
  parts    = lib.splitString "." key;
  attr     = lib.last parts;              # "0"
  resource = lib.concatStringsSep "." (lib.init parts);
```

For `aws.aws_ses_domain_dkim.x.dkim_tokens.0` that asks the ledger for attribute
`0` on a resource called `aws.aws_ses_domain_dkim.x.dkim_tokens`, which does not
exist. The leaf stays a placeholder and never becomes concrete. The comment in
`ref.nix` says as much, calling nested path lookup out of PoC scope, but the
executor has since grown it and the two halves no longer agree.

## The case that hit it

SES DKIM records need the token twice, in the record name and in its value:

```nix
name    = "${token}._domainkey.example.org";
records = [ "${token}.dkim.amazonses.com" ];
```

`mkResources` with `count = 3` expands correctly; the reference inside each
instance is what fails. There is no AWS datasource for DKIM tokens, so the only
way through was writing all three tokens as literals and putting
`preventDestroy` on the identity to keep them true.

Terraform's `element(aws_ses_domain_dkim.x.dkim_tokens, count.index)` is the
idiom this blocks, so it will be hit by anyone porting an SES, subnet, or
availability-zone module.

## Fix

Make `ledgerLookup` walk a path the way `graph.index` already does, rather than
splitting the dotted key and taking the last segment. Both sides then agree on
what a `Path` means, which `internal/ir/types.go` already models as
`[]interface{}`.

Until then `nivis.str` could fail loudly when handed a marker whose path has
more than one element, instead of producing a leaf that silently never
resolves.

## Repro

Any resource with a list output, referenced by index, inside a string:

```nix
records = [ (nivis.str [ (dkim.refPath [ "dkim_tokens" 0 ]) ".dkim.amazonses.com" ]) ];
```

Expected: resolves after the dkim resource is known.
Actual: the `__derived` leaf is never rendered.
