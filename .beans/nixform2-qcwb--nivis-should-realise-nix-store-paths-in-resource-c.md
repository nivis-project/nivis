---
# nixform2-qcwb
title: Nivis should realise Nix store paths in resource config before apply
status: todo
type: feature
priority: normal
tags:
    - discovered
created_at: 2026-06-15T21:46:15Z
updated_at: 2026-06-15T21:46:15Z
parent: nixform2-uqn6
---

When a resource config value is a Nix build output (e.g. aws_s3_object.source = a built .vhd's store path, as in the EC2+NixOS tutorial), Nivis evaluates the flake with 'nix eval' which does NOT realise derivations — so the path is valid-but-unbuilt and the provider fails: 'opening S3 object source (...): no such file or directory'. Today the user must 'nix build' the image before 'nivis apply'. Better UX: have the executor realise (nix-store --realise / nix build) store paths it finds in resource config before applying, so a single 'nivis apply' builds + ships. Discovered 2026-06-15 during the EC2 tutorial (rx5h). Workaround documented in the tutorial: build the image first.
