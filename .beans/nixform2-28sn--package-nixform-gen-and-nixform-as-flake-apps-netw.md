---
# nixform2-28sn
title: Package nixform-gen (and nixform) as flake apps (NETWORK-GATED)
status: todo
type: task
priority: low
tags:
    - discovered
    - network-gated
created_at: 2026-06-15T11:18:03Z
updated_at: 2026-06-15T11:18:03Z
parent: nixform2-dwqg
---

A true 'nix run .#gen' / 'nix run .#nixform' flake app needs nixpkgs buildGoModule (and writeScriptBin to mark the program executable). The Nix binary cache is unreachable here (CLAUDE.md §6), so flake-app packaging is deferred. Today the codegen + executor run via 'go run ./cmd/nixform-gen' and 'go build ./cmd/nixform'. When nixpkgs is reachable, add apps.<system>.{gen,nixform}.
