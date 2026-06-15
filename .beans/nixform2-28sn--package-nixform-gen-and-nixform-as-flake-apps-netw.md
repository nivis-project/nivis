---
# nixform2-28sn
title: Package nixform-gen (and nixform) as flake apps (NETWORK-GATED)
status: completed
type: task
priority: low
tags:
    - discovered
    - network-gated
created_at: 2026-06-15T11:18:03Z
updated_at: 2026-06-15T16:29:24Z
parent: nixform2-dwqg
---

A true 'nix run .#gen' / 'nix run .#nixform' flake app needs nixpkgs buildGoModule (and writeScriptBin to mark the program executable). The Nix binary cache is unreachable here (CLAUDE.md §6), so flake-app packaging is deferred. Today the codegen + executor run via 'go run ./cmd/nixform-gen' and 'go build ./cmd/nixform'. When nixpkgs is reachable, add apps.<system>.{gen,nixform}.


---
DONE (no longer network-gated) via OpenSpec change flake-apps (archived 2026-06-15-flake-apps).
The original premise was stale: this environment has a registry-pinned nixpkgs and a realised Go toolchain, and buildGoModule builds tn offline. Added to flake.nix: inputs.nixpkgs (locked to a github rev in flake.lock; NO flake-utils — a small inline forAllSystems helper enumerates systems), packages.<sys>.{tn,tn-gen,default} via pkgs.buildGoModule (Go from the pinned nixpkgs; module deps pinned by a committed vendorHash, no vendor/ dir), and apps.<sys>.{tn,tn-gen,default}. Verified: `nix build .#tn` ok; `nix run .#tn -- --version` -> "tn (Terrae Nivis) v1.0"; `nix run .#tn-gen -- --help` ok. Purity invariant HELD and proven: `nix eval .#lib` succeeds even with --override-flake nixpkgs path:/nonexistent and NIX_PATH="" — the library outputs never force nixpkgs (DESIGN.md D7). README/getting-started/DESIGN updated; go build/go run still work unchanged. Names updated from old nixform-gen/nixform to tn/tn-gen.
