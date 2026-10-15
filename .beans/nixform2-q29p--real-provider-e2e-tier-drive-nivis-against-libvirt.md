---
# nixform2-q29p
title: 'Real-provider e2e tier: drive nivis against libvirt and postgres in a nixosTest'
status: todo
type: feature
priority: normal
tags:
    - discovered
created_at: 2026-10-15T19:10:28Z
updated_at: 2026-10-15T19:45:24Z
---

A hermetic CI tier that drives nivis against REAL third-party providers whose
backends run inside a nixosTest VM. It sits between the in-repo fakes and the
live AWS tier (nixform2-znyj), and it supersedes that bean's expensive tier.

## Why: every defect this week came from a fake agreeing with us

| bean           | the fake agreed with us about                          |
|----------------|--------------------------------------------------------|
| nixform2-1mk0  | unknown vs null in Config, so schema defaults never fired |
| nixform2-aur6  | which prior the create half of a replacement plans against |
| nixform2-msl4  | which protocol object drives the planned state         |
| nixform2-cjxz  | how requires-replace is decided                        |

All four were found by hand against a production AWS account. A real provider
cannot agree with our misreading: it decides these from its own schema.

## The two providers, and why this pair

**dmacvicar/libvirt** carries AWS's structural complexity:

- nested schemas four deep (`devices.consoles.source`), 12,430 lines of docs
  for the domain resource alone. That is the honest stress test for
  nixform2-p4uz (nested blocks).
- base volume -> COW volume -> domain -> network interface, which is the same
  artifact -> derived artifact -> machine shape as the s3obj -> snapshot -> ami
  -> instance chain that broke nivis in nixform2-nwnf.
- force-new cascading up that chain.
- a value that arrives AFTER apply returns: the wait-for-lease mechanism
  (`network` CIDR to wait for, `source: lease|agent|any`). No in-repo fake can
  express async.

**cyrilgdn/postgresql** carries the one thing libvirt does not, and it is the
hard case in nixform2-3utj (import): an attribute the provider NEVER returns.
Verified in `postgresql/resource_postgresql_role.go`: `password` is Sensitive,
`Read` never sets it, and the resource carries a third attribute whose whole
purpose is "prevents applies from updating the role password on every apply
unless the value changes". The provider's own workaround for the problem is
visible in its schema.

Together they also satisfy the milestone exit criterion in CLAUDE.md (two
providers, unknowns on both sides, a Nix-side consumer reading from both) with
REAL providers rather than alpha/beta.

## Measured, 2026-10-15

All candidates are served by registry.opentofu.org, and their backends are in
nixpkgs with existing nixosTests:

| provider             | backend in nixpkgs | nixos tests |
|----------------------|--------------------|-------------|
| dmacvicar/libvirt    | libvirt 12.2.0     | 2           |
| cyrilgdn/postgresql  | postgresql 17.10   | 2           |
| lxc/incus            | incus 7.3.0        | 10          |
| aminueza/minio       | minio              | 1           |
| hashicorp/vault      | vault 1.21.4       | 1           |

All report tfprotov5, which also exercises nivis's v5 backend. That path got its
first spawned replacement test only on 2026-10-09.

Driving a third-party provider also exercises the registry path end to end
(resolve, download, verify, cache), which today only `nivis.aws` reaches and
which therefore needs credentials.

## The open bet

Whether libvirtd under QEMU TCG (no nested KVM) reaches a DHCP lease in
acceptable time inside a nixosTest. Everything rests on it. Half-day spike.

Fallback if it does not: **lxc/incus**. Containers, no nested virt, the most
nixosTests of any candidate (10), keeps the image -> instance chain and the
computed `ipv4_address`. It loses the deep nesting (447 doc lines against
libvirt's 12,430), so postgres would carry more of the weight.

## Three tiers, deliberately

1. in-repo fakes: fast, seeded, deterministic, run inside the `nix flake check`
   sandbox. Unchanged, stays tier 1.
2. THIS: real providers, hermetic, CI only, slow.
3. nixform2-znyj, re-scoped: real S3/KMS/STS against AWS with FAKE providers,
   for SigV4, KMS key policies and STS assume-role. Nothing here touches those.

## Two things to be deliberate about

- Async is what this tests and also what makes tests flake. A bounded wait with
  a message distinguishing "nivis got it wrong" from "the guest was slow", or it
  costs more trust than it earns.
- The two providers do not naturally depend on each other. Either a synthetic
  Nix-side consumer combining both outputs (what the headline test already
  does), or postgres inside the libvirt guest (nested-nested; do not). Decide up
  front rather than by accident.

## Alternatives considered and rejected

- **fakecloud** (Rust AWS emulator): AGPL-3.0 against nivis's Apache-2.0, not in
  nixpkgs, no nix expression. Packaging is easy (Cargo.lock, static musl
  releases) but the licence is a standing question for a flake input.
- **MiniStack** (Python AWS emulator, MIT): 38 PyPI deps with hard pins
  (`botocore==1.43.106`), no nix expression, heavier runtime. uv2nix territory
  plus maintenance on every pin bump.
- Both share a deeper problem: an emulator is someone else's understanding of
  AWS. A real postgres or libvirt has no fidelity gap at all, because it IS the
  service. The emulators remain the better answer only for AWS API surface
  without an account.
- **Golden corpus** (record real AWS responses, replay hermetically), proposed
  in nixform2-znyj: better aimed at "nivis sends the wrong bytes" and cheaper,
  but it cannot respond to a request it has no recording for, so it cannot
  exercise a replacement cascade unless one was recorded, and it drifts as AWS
  changes. Complementary; kept with znyj.


## Also the import substrate (nixform2-3utj)

Verified in source on 2026-10-15, not assumed.

Both providers implement import. `cyrilgdn/postgresql` uses
`Importer: &schema.ResourceImporter{StateContext: schema.ImportStatePassthroughContext}`
on `postgresql_role` and `postgresql_database`; `dmacvicar/libvirt` has
`ImportState` on domain, volume, network and pool resources.

Postgres carries almost the whole import bean on its own:

| 3utj requirement                          | vehicle                      |
|-------------------------------------------|------------------------------|
| 1. attributes a provider never returns    | `postgresql_role.password`: Sensitive, `Read` never sets it. Import a role and the plan is NOT clean, which is the failure the seeding mechanism must fix |
| 2. identity is not the id attribute       | postgres imports by role or database NAME; libvirt by UUID or path |
| 3. bulk, not one call per resource        | create 50 roles out of band with SQL, import via a manifest, assert one clean plan. Seconds, deterministic |
| 4. acceptance is a clean plan             | `PlanReport` already returns ops, so "every op is NoOp" is assertable |

`ImportStatePassthroughContext` matters beyond the checkbox: it sets the id
from the import string and relies on READ to populate the rest. That is exactly
3utj's first open question ("does refresh populate attributes correctly from a
minimal attrs containing only the identity?"), answerable here with no AWS
account, because the provider's Read queries `pg_roles` by name.

Libvirt's contribution to import is narrower: complex identities and the async
case, which matter for "a wrong import must not quietly propose a destroy".
The bulk and never-returned-attribute cases do not need it.

### Two things this tier will surface, deliberately or otherwise

**Write-only attributes.** `postgresql_role` declares `password_wo` with
`WriteOnly: true`, plus `password_wo_version` whose stated purpose is
"prevents applies from updating the role password on every apply unless the
value changes". Write-only is a protocol feature and there is no evidence nivis
handles it. Worth testing on purpose rather than meeting it as a confusing
failure.

**The schema-version hole stays open.** `resource_postgresql_role.go` has no
`SchemaVersion` and no `StateUpgraders`, so this pair cannot exercise the
state-upgrade path at all. See 3utj: nivis never calls
`UpgradeResourceState` and `state.ResourceState` records no version.
Importing a Terraform state written at an older provider schema remains
unexercised. It needs a provider with a version bump, or it is a known hole.


## Split into two halves (2026-10-15)

The two halves carry completely different risk and the import work depends only
on the first, so they are separate beans under this one. This bean stays as the
umbrella holding the research.

- **postgres half**: no bet. postgresql is in nixpkgs with two existing
  nixosTests, the provider is on the registry at protocol 5. Mostly wiring.
  Answers four things empirically that would otherwise be reasoned about in
  nixform2-3utj: whether nivis handles `WriteOnly` at all, 3utj's open
  question 1 (does refresh populate from minimal attrs), whether the registry
  path works for a non-AWS provider, and whether the clean-plan assertion is
  mechanically sound against a real provider.
- **libvirt half**: carries the whole TCG / DHCP-lease bet and the incus
  fallback decision.

Substrate before design: reasoning about whether nivis handles a protocol
feature is exactly what produced nixform2-1mk0, aur6, msl4 and cjxz. A real
provider settles it.
