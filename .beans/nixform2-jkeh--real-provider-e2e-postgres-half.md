---
# nixform2-jkeh
title: 'Real-provider e2e: postgres half'
status: completed
type: task
priority: normal
created_at: 2026-10-15T19:45:43Z
updated_at: 2026-10-15T20:30:00Z
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


## Done 2026-10-15

OpenSpec change `real-postgres-provider-e2e`. The harness exists: a nixosTest
boots postgresql, nivis evaluates the flake OFFLINE, spawns the real
`cyrilgdn/postgresql` 1.26.0 from a store path, creates a role, and the plan
straight after is clean.

**The premise held: no file under `internal/` or `cmd/` changed.** nivis
already drove a real third-party provider; only the harness was missing.

### Runtime, for the per-push versus scheduled decision

**13.22 seconds** for the VM script. Per push is comfortably the right call; the
fallback to a schedule is not needed.

### What it covers, and what it deliberately does not

Covers: nivis against an unmodified third-party provider, offline, with no
credentials; the clean-plan acceptance criterion; and a drift subtest proving
that criterion is a real signal rather than a constant.

Does NOT cover, each kept out so the harness could be judged on its own:

- the dependency chain (role -> database -> schema -> grant)
- write-only attributes: nixform2-l0th
- import: nixform2-3utj
- libvirt and its TCG bet: nixform2-l27s
- the registry path. A filesystem source bypasses it by design, which corrects
  one of the four questions this bean originally claimed to answer.

### One thing learned

The first run failed on `password authentication failed for user postgres`:
the provider connects over TCP and matched the default `::1/128 md5` rule, not
the unix socket the fixture assumed. Everything before that already worked,
including the offline flake evaluation, which was the part most likely to fail.


## Summary of Changes

A nixosTest drives nivis against the real, unmodified `cyrilgdn/postgresql`
1.26.0 provider and a real postgresql, inside a VM with no network and no
credentials. Four subtests:

1. nivis applies against the real provider
2. the role exists in postgresql, queried through psql rather than through nivis
3. the plan straight after applying reports no changes
4. that clean plan is a real signal: altering the role out of band makes the
   next plan see it

The fourth exists because task 4 could not be done as written. "Break the
fixture" is impossible when the same fixture is both applied and planned: it
stays self-consistent and the plan stays clean either way. Drift is the honest
falsification, and without it the third subtest would also pass for a nivis that
reported "No changes" unconditionally. Kept permanently rather than run once.

### The premise held

**No file under `internal/` or `cmd/` changed.** The proposal rested on nivis
already driving a real third-party provider, and it did. Only the harness was
missing. If that had turned out false, the design said to stop rather than work
around it.

### Where it runs

Outside `checks`, as its own CI step. `nix flake check`'s set is identical
before and after (`build, coverage, fake-providers, go-tests, gofmt`), so
`scripts/ship-change.sh` is exactly as fast as before. The VM script takes
**13.22 seconds**, so per push is comfortably right and the scheduled fallback
is not needed.

### Verification

VM test green. `nix flake check` passes. `go test ./...` green and untouched.
The provider is a store path, so the registry is deliberately not exercised.

One failure on the way: the first run died on `password authentication failed
for user postgres`, because the provider connects over TCP and matched the
default `::1/128 md5` rule rather than the unix socket the fixture assumed.
Everything before that already worked, including the offline flake evaluation,
which was the part most likely to fail.

OpenSpec change: `real-postgres-provider-e2e`.
