{
  description = "Nivis — Terraform/OpenTofu provider resources as first-class Nix values";
  inputs.nixpkgs.url = "nixpkgs";

  outputs =
    { self, nixpkgs }:
    let

      # The pure library: builtins only, no nixpkgs. Evaluating `lib` / `nivis.*`
      # never forces the nixpkgs input.
      nivis = import ./nix/lib { };

      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "x86_64-darwin"
        "aarch64-darwin"
      ];
      forAllSystems = f: builtins.listToAttrs (map (system: { name = system; value = f system; }) systems);
      pkgsFor = system: import nixpkgs { inherit system; };

      # buildGoModule package for the nivis CLI (codegen is the `nivis gen`
      # subcommand). Go toolchain from the pinned nixpkgs; deps pinned by
      # vendorHash (no vendor/ dir). If go.mod changes, `nix build` reports the
      # expected hash; update it here.
      # Single source of truth for the version: the top-level VERSION file.
      version = nixpkgs.lib.fileContents ./VERSION;

      mkCli =
        system:
        let
          pkgs = pkgsFor system;
        in
        pkgs.buildGoModule {
          pname = "nivis";
          inherit version;
          src = ./.;
          vendorHash = "sha256-LjGLaFdEYWqe42JHhRG1IzGqFn7yobbhIeLJ/Enc+l4=";
          subPackages = [ "cmd/nivis" ];
          # Inject the canonical version into the binary (overrides the "dev"
          # default in cmd/nivis). -s -w strip debug info.
          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];
          meta = {
            description = "Terraform/OpenTofu provider resources as first-class Nix values";
            mainProgram = "nivis";
          };
        };
    in
    {
      # The public library, for `import`/`lib` consumers. Pure builtins.
      lib = nivis;

      # nivis.plan is a function of the injected outputs ledger (empty on phase
      # 0). The phased-eval driver calls:
      #   nix eval .#nivis.plan --apply 'plan: plan { outputs = <ledger>; }' --json
      # and feeds the resulting IR to the executor. Pure builtins — no nixpkgs.
      nivis = {
        plan = import ./nix/example { inherit nivis; };
        # A cyclic variant for the headline e2e's cycle-rejection assertion.
        planCycle = import ./nix/example/cycle.nix { inherit nivis; };
        # A real-provider example (AWS S3 bucket) — drive with `nivis ... --attr
        # nivis.aws`; creates a real resource (see nix/example/aws.nix).
        aws = import ./nix/example/aws.nix { inherit nivis; };
      };

      # CLI package: `nix build .#nivis`.
      packages = forAllSystems (
        system:
        let
          cli = mkCli system;
        in
        {
          nivis = cli;
          default = cli;
        }
      );

      # CLI app: `nix run .#nivis -- …` (codegen: `nix run .#nivis -- gen …`).
      apps = forAllSystems (
        system:
        let
          cli = mkCli system;
        in
        {
          nivis = {
            type = "app";
            program = "${cli}/bin/nivis";
          };
          default = {
            type = "app";
            program = "${cli}/bin/nivis";
          };
        }
      );
    };
}
