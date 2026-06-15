{
  description = "terrae nivis — Terraform/OpenTofu provider resources as first-class Nix values";
  inputs.nixpkgs.url = "nixpkgs";

  outputs =
    { self, nixpkgs }:
    let

      # The pure library: builtins only, no nixpkgs. Evaluating `lib` /
      # `terraeNivis.*` never forces the nixpkgs input.
      terraeNivis = import ./nix/lib { };

      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = f: builtins.listToAttrs (map (system: { name = system; value = f system; }) systems);
      pkgsFor = system: import nixpkgs { inherit system; };

      # buildGoModule package for the two user-facing CLIs. Go toolchain comes from
      # the pinned nixpkgs; module deps are pinned by vendorHash (no vendor/ dir).
      # If go.mod changes, `nix build` reports the expected hash; update it here.
      # Single source of truth for the version: the top-level VERSION file.
      # fileContents strips the trailing newline.
      version = nixpkgs.lib.fileContents ./VERSION;

      mkCli =
        system:
        let
          pkgs = pkgsFor system;
        in
        pkgs.buildGoModule {
          pname = "terrae-nivis";
          inherit version;
          src = ./.;
          vendorHash = "sha256-LjGLaFdEYWqe42JHhRG1IzGqFn7yobbhIeLJ/Enc+l4=";
          subPackages = [
            "cmd/tn"
            "cmd/tn-gen"
          ];
          # Inject the canonical version into the binary (overrides the "dev"
          # default in cmd/tn). -s -w strip debug info.
          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];
          meta = {
            description = "Terraform/OpenTofu provider resources as first-class Nix values";
            mainProgram = "tn";
          };
        };
    in
    {
      # The public library, for `import`/`lib` consumers. Pure builtins.
      lib = terraeNivis;

      # terraeNivis.plan is a function of the injected outputs ledger (empty on phase
      # 0). The phased-eval driver (E3.5) calls:
      #   nix eval .#terraeNivis.plan --apply 'plan: plan { outputs = <ledger>; }' --json
      # and feeds the resulting IR to the executor. Pure builtins — no nixpkgs.
      terraeNivis = {
        plan = import ./nix/example { inherit terraeNivis; };
        # A cyclic variant for the headline e2e's cycle-rejection assertion.
        planCycle = import ./nix/example/cycle.nix { inherit terraeNivis; };
        # A real-provider example (AWS S3 bucket) — drive with `tn ... --attr
        # terraeNivis.aws`; creates a real resource (see nix/example/aws.nix).
        aws = import ./nix/example/aws.nix { inherit terraeNivis; };
      };

      # CLI packages: `nix build .#tn` / `.#tn-gen`. The single derivation builds
      # both binaries; the per-name attrs select the program via meta.mainProgram /
      # the app wrappers below.
      packages = forAllSystems (
        system:
        let
          cli = mkCli system;
        in
        {
          tn = cli;
          tn-gen = cli;
          default = cli;
        }
      );

      # CLI apps: `nix run .#tn -- …` / `nix run .#tn-gen -- …`.
      apps = forAllSystems (
        system:
        let
          cli = mkCli system;
          app = name: {
            type = "app";
            program = "${cli}/bin/${name}";
          };
        in
        {
          tn = app "tn";
          tn-gen = app "tn-gen";
          default = app "tn";
        }
      );
    };
}
