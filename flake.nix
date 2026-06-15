{
  description = "nixform — Terraform/OpenTofu provider resources as first-class Nix values";

  # No external inputs: the Nix library is pure builtins (no nixpkgs), so it
  # evaluates without the binary cache (CLAUDE.md §6). The Go side is built with
  # the standard Go toolchain, not via this flake, for the PoC.
  inputs = { };

  outputs =
    { self }:
    let
      nixform = import ./nix/lib { };
    in
    {
      # The public library, for `import`/`lib` consumers.
      lib = nixform;

      # nixform.plan is a function of the injected outputs ledger (empty on phase
      # 0). The phased-eval driver (E3.5) calls:
      #   nix eval .#nixform.plan --apply 'plan: plan { outputs = <ledger>; }' --json
      # and feeds the resulting IR to the executor.
      nixform = {
        plan = import ./nix/example { inherit nixform; };
      };
    };
}
