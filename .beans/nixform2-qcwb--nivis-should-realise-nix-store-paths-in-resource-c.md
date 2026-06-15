---
# nixform2-qcwb
title: Nivis should realise Nix store paths in resource config before apply
status: completed
type: feature
priority: normal
tags:
    - discovered
created_at: 2026-06-15T21:46:15Z
updated_at: 2026-06-15T22:52:57Z
parent: nixform2-uqn6
---

When a resource config value is a Nix build output (e.g. aws_s3_object.source = a built .vhd's store path, as in the EC2+NixOS tutorial), Nivis evaluates the flake with 'nix eval' which does NOT realise derivations — so the path is valid-but-unbuilt and the provider fails: 'opening S3 object source (...): no such file or directory'. Today the user must 'nix build' the image before 'nivis apply'. Better UX: have the executor realise (nix-store --realise / nix build) store paths it finds in resource config before applying, so a single 'nivis apply' builds + ships. Discovered 2026-06-15 during the EC2 tutorial (rx5h). Workaround documented in the tutorial: build the image first.


---
DONE via OpenSpec change auto-build-derivations (archived 2026-06-15-auto-build-derivations). nivis now builds what it needs: a `nivis.drv` helper emits a __build leaf ({__build:{path}}); the executor realises it per-resource in applyOne (nix-store --realise the store root) before apply and substitutes the path. Per-phase, only-what's-ready, so a build depending on an earlier resource's output is realised in a later phase — the build participates in the fixpoint (the A->a->B->b->C model). --no-build opt-out; build failure names the path. __build is a KNOWN leaf (not an edge/unknown), passes through resolve, conforms to ir-schema. Migrated nix/example/ec2.nix + the tutorial to `drv` (dropped manual passthru.filePath AND the separate `nix build` step). VERIFIED LIVE (AWS 076504012268): a SINGLE `nivis apply` built the NixOS image itself, uploaded/imported/registered/launched it, instance served HTTP 200 ("hello from NixOS on EC2, built and launched by Nivis"); destroyed all 9, zero orphans. Tests: Nix P8 (__build leaf), 4 Go unit (substitute/no-build/failure/storeRoot). Closes the anti-pattern.
