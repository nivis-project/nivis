# Spec delta: nix-lib

## ADDED Requirements

### Requirement: Flake apps build and run the CLIs
The flake SHALL expose `packages.<system>.{tn,tn-gen}` (with `default = tn`) that
build the `tn` and `tn-gen` binaries from source via nixpkgs `buildGoModule`
(Go toolchain from the pinned `nixpkgs` input; module deps pinned by a committed
`vendorHash`, no `vendor/` directory), and matching `apps.<system>.{tn,tn-gen}`
(with `default = tn`) so `nix run .#tn -- …` and `nix run .#tn-gen -- …` work.
System enumeration SHALL use a small inline helper, not `flake-utils`. Adding
these outputs SHALL NOT make the library outputs depend on nixpkgs: `lib` and
`terraeNivis.*` SHALL still evaluate without forcing the nixpkgs input.

#### Scenario: nix run drives the CLI
- WHEN `nix run .#tn -- --version` is executed
- THEN it builds `tn` from source and prints the version.

#### Scenario: the library still evaluates input-free
- WHEN `nix eval .#lib` and `nix eval .#terraeNivis.plan --apply 'p: p { phase = 0; outputs = {}; }'` are evaluated
- THEN they succeed without building or importing anything from the nixpkgs input
  (the library remains pure-builtins).
