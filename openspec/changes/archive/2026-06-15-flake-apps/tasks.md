# Tasks: flake-apps

## 1. Spec
- [x] 1.1 Write proposal, tasks, nix-lib spec delta (ADDED flake-apps requirement; library still evaluates without forcing nixpkgs)
- [x] 1.2 `openspec validate flake-apps` passes

## 2. Implement flake.nix
- [x] 2.1 Add `inputs.nixpkgs.url = "nixpkgs"`; inline `forAllSystems` helper (no flake-utils); `pkgsFor.<system>`
- [x] 2.2 `packages.<system>.{tn,tn-gen,default}` via `pkgs.buildGoModule` (Go from pinned nixpkgs, vendorHash, no vendor/)
- [x] 2.3 `apps.<system>.{tn,tn-gen,default}` wrapping the packages
- [x] 2.4 `lib` / `terraeNivis.*` unchanged (do not depend on the nixpkgs input)
- [x] 2.5 Generate `flake.lock` (locks offline via the registry-pinned nixpkgs)

## 3. Docs
- [x] 3.1 README + getting-started: `nix run .#tn` / `.#tn-gen` alongside go build/run
- [x] 3.2 DESIGN.md note: apps use the nixpkgs input; library outputs stay input-free

## 4. Verify + close
- [x] 4.1 `nix run .#tn -- --version` and `nix run .#tn-gen -- --help` work
- [x] 4.2 `nix build .#tn` succeeds
- [x] 4.3 `nix eval .#lib` / `.#terraeNivis.plan` succeed (library evaluates, doesn't force nixpkgs)
- [x] 4.4 `openspec archive flake-apps`; fold requirement into nix-lib spec
- [x] 4.5 Close beans-28sn; commit as Pim Snel; push
