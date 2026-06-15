{
  description = "terrae nivis — Terraform/OpenTofu provider resources as first-class Nix values";

  # No external inputs: the Nix library is pure builtins (no nixpkgs), so it
  # evaluates without the binary cache (CLAUDE.md §6). The Go side is built with
  # the standard Go toolchain, not via this flake, for the PoC.
  inputs = { };

  outputs =
    { self }:
    let
      terraeNivis = import ./nix/lib { };
    in
    {
      # The public library, for `import`/`lib` consumers.
      lib = terraeNivis;

      # terraeNivis.plan is a function of the injected outputs ledger (empty on phase
      # 0). The phased-eval driver (E3.5) calls:
      #   nix eval .#terraeNivis.plan --apply 'plan: plan { outputs = <ledger>; }' --json
      # and feeds the resulting IR to the executor.
      terraeNivis = {
        plan = import ./nix/example { inherit terraeNivis; };
        # A cyclic variant for the headline e2e's cycle-rejection assertion.
        planCycle = import ./nix/example/cycle.nix { inherit terraeNivis; };
        # A real-provider example (AWS S3 bucket) — drive with `tn ... --attr
        # terraeNivis.aws`; creates a real resource (see nix/example/aws.nix).
        aws = import ./nix/example/aws.nix { inherit terraeNivis; };
      };
    };
}
