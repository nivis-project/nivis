---
# nixform2-jkeh
title: 'Real-provider e2e: postgres half'
status: todo
type: task
priority: normal
created_at: 2026-10-15T19:45:43Z
updated_at: 2026-10-15T19:47:35Z
parent: nixform2-q29p
---

The no-bet half of nixform2-q29p. See that bean for the research and the full
candidate comparison.

Drive nivis against the REAL `cyrilgdn/postgresql` provider with a real
postgresql running inside a nixosTest. postgresql is in nixpkgs with two
existing nixosTests and the provider is on registry.opentofu.org at protocol 5,
so this is wiring rather than a gamble.

## What it answers, empirically

Each of these would otherwise be reasoned about while designing import
(nixform2-3utj), and reasoning is what produced nixform2-1mk0, aur6, msl4 and
cjxz.

| question | how it gets answered |
|----------|----------------------|
| does nivis handle `WriteOnly` attributes at all | drive `postgresql_role` with `password_wo` and watch |
| 3utj open question 1: does refresh populate attributes from a minimal attrs containing only the identity | `ImportStatePassthroughContext` sets the id and relies on Read; postgres Read queries `pg_roles` by name |
| does the registry path work for a non-AWS provider | resolve, download, verify, cache, with no credentials |
| is the clean-plan assertion mechanically sound | assert every op is NoOp against a real provider. 3utj's acceptance criterion has never been asserted against one |

## Also

- `postgresql_role.password` is Sensitive and `Read` never sets it: a real
  never-returned attribute, which is 3utj requirement 1.
- Protocol 5 exercises nivis's v5 backend, which got its first spawned
  replacement test only on 2026-10-09.
- Dependency chain for the phased loop: role -> database -> schema -> grant.

## Known obstacle to check first

A nixosTest runs in the Nix build sandbox with no network, so the provider
binary cannot be fetched at run time. nivis's registry spec has a scenario for a
`source` that is a filesystem path being used directly with no registry
interaction; if that holds, the provider is packaged as a derivation and nivis
is pointed at the path. Verify before designing anything.

Note the tension with the table above: a path source means the registry path is
NOT exercised by this test. Those are two different tests and should not be
conflated.


## Spiked 2026-10-15: both enablers clear, and one finding

### The obstacle is not an obstacle

`internal/registry/resolver.go:15` does `os.Stat(source)` and uses an
existing path verbatim, with no registry interaction. A `Manager` with no
resolver uses sources verbatim too. So a nixosTest can point nivis at a store
path and the no-network sandbox is a non-issue.

`nixpkgs#terraform-providers.postgresql` is packaged (1.26.0, cyrilgdn). The
binary lands at
`libexec/terraform-providers/registry.terraform.io/cyrilgdn/postgresql/1.26.0/linux_amd64/`.

### nivis drives the real provider today

Spiked with a throwaway test against the built binary: nivis spawned it,
negotiated, listed **15 resource types** and fetched the `postgresql_role`
schema. No code change needed to reach that point. The probe was removed; it
belongs in the real test.

### The finding: nivis drops WriteOnly, and never opts in

The protocol carries write-only attributes in TWO places, both present in
nivis's own generated protobuf for v5 and v6:

- `ClientCapabilities.WriteOnlyAttributesAllowed` (`tfplugin5.pb.go:842`):
  the CLIENT must opt in.
- `Schema_Attribute.WriteOnly` (`tfplugin5.pb.go:2453`): the per-attribute
  flag. Its doc: "the attribute value will be provided via configuration and
  must be omitted from plan and state response objects".

nivis references neither outside the generated code. It never sends
`ClientCapabilities` at all, and it drops the schema flag.

So this is a negotiation nivis does not participate in. That matters beyond
import: a provider that sees no opt-in may behave differently from one that
does, and nivis has never exercised either path.

### Consequence for the test

A path source means the REGISTRY path is not exercised here. That was listed as
one of the four questions this half answers; it is not. Resolve, download,
verify and cache need their own test with network, which is a different tier.
The other three stand.
