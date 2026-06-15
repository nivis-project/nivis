# Proposal: flake-apps

## Why
`tn` and `tn-gen` are run via `go build`/`go run` today; the flake exposes only
the pure library + IR (`lib`, `terraeNivis.*`). beans-28sn asked for `nix run`
flake apps but parked them as **network-gated** — a `buildGoModule` app was
assumed to need the unreachable binary cache. **That assumption no longer
holds:** this environment has a pinned `nixpkgs` in the system flake registry
(resolvable to a store path, so it locks offline) and a realised Go toolchain,
and `buildGoModule` was verified to build `tn` offline (tests pass) here. So the
apps are now buildable and worth adding.

## What changes
- **`flake.nix`: add a `nixpkgs` input and `packages`/`apps`; no `flake-utils`.**
  - Add `inputs.nixpkgs.url = "nixpkgs"` (resolves via the registry; pinned in a
    new `flake.lock`). The **Go toolchain comes from this pinned nixpkgs**, not an
    ambient `<nixpkgs>` — hermetic and reproducible.
  - Replace flake-utils with a few lines of Nix: a small inline `forAllSystems`
    helper mapping over a fixed system list, building `pkgsFor.<system> =
    import nixpkgs { inherit system; }`.
  - `packages.<system>.{tn,tn-gen}` (and `default = tn`) build the binaries with
    `pkgs.buildGoModule { src = ./.; vendorHash = "<sha256>"; subPackages =
    [...]; }`. `vendorHash` is committed (no `vendor/` dir in the repo).
  - `apps.<system>.{tn,tn-gen}` (and `default = tn`) wrap those packages as
    `{ type = "app"; program = "${pkg}/bin/<name>"; }`, so `nix run .#tn -- …`
    and `nix run .#tn-gen -- …` work.
- **Purity preserved where it matters.** `lib` and `terraeNivis.*` stay
  pure-builtins and do **not** depend on the nixpkgs input — evaluating them
  imports nothing from nixpkgs (only the new `packages`/`apps` do). The invariant
  shifts from "no inputs at all" to "the library outputs don't force nixpkgs"; a
  DESIGN.md note records the refinement and why (apps need a real toolchain).
- **Docs.** README/getting-started note `nix run .#tn` / `.#tn-gen` as an
  alternative to `go build`/`go run`.

## Non-goals
- `flake-utils` — replaced by a small inline system-enumeration helper.
- A committed `vendor/` directory — using `vendorHash` keeps the repo lean.
- Packaging the fake providers as apps — only the user-facing `tn`/`tn-gen`.
- A NixOS module / overlay, or CI for `nix build` — out of scope.

## Impact
- Changed: `flake.nix` gains an `inputs.nixpkgs` + `packages`/`apps`; a new
  `flake.lock` pins nixpkgs. README, getting-started, DESIGN.md notes.
- Verification: `nix run .#tn -- --version` and `nix run .#tn-gen -- --help`
  work; `nix build .#tn` succeeds; **`nix eval .#lib` / `.#terraeNivis.plan`
  succeed** (the library still evaluates and does not force the nixpkgs input).
- If `vendorHash` drifts when `go.mod` changes, `nix build` fails with the
  expected/got hash; the fix is a one-line update (documented).
- Closes beans-28sn.
