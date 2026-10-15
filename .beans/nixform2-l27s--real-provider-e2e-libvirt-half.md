---
# nixform2-l27s
title: 'Real-provider e2e: libvirt half'
status: todo
type: task
priority: normal
created_at: 2026-10-15T19:45:43Z
updated_at: 2026-10-15T19:52:36Z
parent: nixform2-q29p
---

The half of nixform2-q29p that carries the bet. See that bean for the research.

libvirt is what brings AWS's structural complexity: schemas nested four deep
(12,430 lines of docs for the domain resource alone), a base volume -> COW
volume -> domain chain matching the topology that broke nivis in nixform2-nwnf,
force-new cascading up that chain, and a value arriving after apply returns via
the wait-for-lease mechanism. No in-repo fake can express async.

## The bet

Whether libvirtd under QEMU TCG (no nested KVM) reaches a DHCP lease in
acceptable time inside a nixosTest. Everything here rests on it. Half-day spike,
and it should be run before any of this is designed.

libvirtd does run in a nixosTest (two exist in nixpkgs) and can fall back to TCG
without KVM. nivis does not care whether the guest does anything useful, only
that it boots far enough to take a lease.

## Fallback

`lxc/incus`: containers, no nested virt, the most nixosTests of any candidate
(10), keeps the image -> instance chain and the computed `ipv4_address`. It
loses the deep nesting (447 doc lines against libvirt's 12,430), so the postgres
half would carry more of the weight.

## Deliberate, not accidental

- Async is what this tests and also what makes tests flake. A bounded wait, and
  a failure message distinguishing "nivis got it wrong" from "the guest was
  slow", or it costs more trust than it earns.
- The two providers do not naturally depend on each other. Either a synthetic
  Nix-side consumer combining both outputs (what the headline test already
  does), or postgres inside the libvirt guest (nested-nested, do not).


## No write-only attributes here (measured 2026-10-15)

libvirt has none: 0 hits for `WriteOnly` in the provider source, and 0 in the
12,430 lines of domain docs. The only "sensitive" mention is
`launch_security`, a feature description rather than an attribute flag.

That fits what it manages. Its whole resource set is infrastructure shape, not
credentials: `cloudinit_disk`, `combustion`, `domain`, `ignition`,
`network`, `pool`, `volume`. A local hypervisor issues no secrets.

So the division of labour with the postgres half is now measured rather than
asserted: libvirt brings nesting, the cascade and async; it contributes nothing
to nixform2-l0th or to nixform2-3utj requirement 1.
