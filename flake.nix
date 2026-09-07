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

      # The NixOS amazon image for the EC2 example: a machine that serves HTTP 200
      # on :80. Built for x86_64-linux (the AMI target). This is the "OS in Nix"
      # half of nix/example/ec2.nix's two-domain mix. Evaluating `nivis.ec2`
      # forces nixpkgs (it builds this) — unlike the pure `lib`/`nivis.plan`.
      ec2NixosImage =
        (nixpkgs.lib.nixosSystem {
          system = "x86_64-linux";
          modules = [
            (
              { modulesPath, ... }:
              {
                imports = [ (modulesPath + "/virtualisation/amazon-image.nix") ];
                services.nginx.enable = true;
                services.nginx.virtualHosts."_".locations."/".return = ''200 "hello from NixOS on EC2, built and launched by Nivis\n"'';
                networking.firewall.allowedTCPPorts = [ 80 ];
                system.stateVersion = "25.05";
              }
            )
          ];
        }).config.system.build.images.amazon;

      # buildGoModule package for the nivis CLI (codegen is the `nivis gen`
      # subcommand). Go toolchain from the pinned nixpkgs; deps pinned by
      # vendorHash (no vendor/ dir). If go.mod changes, `nix build` reports the
      # expected hash; update it here.
      # Single source of truth for the version: the top-level VERSION file.
      version = nixpkgs.lib.fileContents ./VERSION;

      # Go module dependency hash, shared by every buildGoModule call below (they
      # all build the same go.mod). If go.mod changes, `nix build` reports the
      # expected hash; update it here, in this one place.
      goVendorHash = "sha256-Isia+zmQtZlLoS1Kvs1KnCDLLTQ/VsEzmPWdZKUL1BA=";

      mkCli =
        system:
        let
          pkgs = pkgsFor system;
        in
        pkgs.buildGoModule {
          pname = "nivis";
          inherit version;
          src = ./.;
          vendorHash = goVendorHash;
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

      # The in-repo fake tfprotov6 providers (hermetic test substrate), packaged
      # so the offline tutorials can build them with Nix instead of `go build`:
      # `nix build .#fake-providers` -> ./result/bin/provider-{alpha,beta}.
      # They are a test/tutorial aid, not a published artifact.
      mkFakeProviders =
        system:
        let
          pkgs = pkgsFor system;
        in
        pkgs.buildGoModule {
          pname = "nivis-fake-providers";
          inherit version;
          src = ./.;
          vendorHash = goVendorHash;
          subPackages = [
            "cmd/provider-alpha"
            "cmd/provider-beta"
            "cmd/provider-epsilon"
          ];
          ldflags = [
            "-s"
            "-w"
          ];
          meta.description = "Nivis in-repo fake tfprotov6 providers (test/tutorial substrate)";
        };

      # The tutorial scaffolder (flake app `#tutor`). It bundles `nivistutor`
      # together with the fake providers in one /bin, so a single
      # `nix shell …#nivis …#tutor` carries nivis + nivistutor + the providers a
      # scaffolded tutorial needs on PATH. The starter files are embedded in the
      # binary (go:embed), so this is offline and version-locked to the build.
      mkTutor =
        system:
        let
          pkgs = pkgsFor system;
        in
        pkgs.buildGoModule {
          pname = "nivistutor";
          inherit version;
          src = ./.;
          vendorHash = goVendorHash;
          subPackages = [
            "cmd/nivistutor"
            "cmd/provider-alpha"
            "cmd/provider-beta"
            "cmd/provider-epsilon"
          ];
          # Inject the canonical version: nivistutor reports it and pins a
          # scaffolded starter's nivis input to this release (v<version>).
          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
          ];
          meta = {
            description = "Nivis tutorial scaffolder (and the fake providers it needs on PATH)";
            mainProgram = "nivistutor";
          };
        };
      # `nix flake check` gates: the module builds, the whole test suite passes,
      # coverage holds its floors, and the tree is gofmt-clean. These run in the
      # build sandbox (no network, no writable $HOME), which is why the
      # Nix-library tests (tests/run-nix-tests.sh, which needs `nix eval`) and the
      # docs checks (which need mdbook) are NOT here: scripts/ship-change.sh runs
      # those alongside `nix flake check`.
      #
      # A Go check builds nothing and installs nothing — the check IS the run.
      mkGoCheck =
        system:
        { name, phase }:
        let
          pkgs = pkgsFor system;
        in
        pkgs.buildGoModule {
          pname = name;
          inherit version;
          src = ./.;
          vendorHash = goVendorHash;
          buildPhase = ''
            runHook preBuild
            runHook postBuild
          '';
          doCheck = true;
          checkPhase = ''
            runHook preCheck
            ${phase}
            runHook postCheck
          '';
          installPhase = ''
            runHook preInstall
            touch $out
            runHook postInstall
          '';
        };

      mkChecks =
        system:
        let
          pkgs = pkgsFor system;
        in
        {
          # The CLI and the fake providers actually compile.
          build = mkCli system;
          fake-providers = mkFakeProviders system;

          # The whole Go suite. The e2e tests skip themselves when `nix` is not on
          # PATH (it is not, in the sandbox); everything else runs.
          go-tests = mkGoCheck system {
            name = "nivis-go-tests";
            phase = "go test ./...";
          };

          # Coverage floors — scripts/coverage.sh documents the ratchet and why
          # the generated protobuf packages are excluded from the overall number.
          coverage = mkGoCheck system {
            name = "nivis-coverage";
            phase = "bash scripts/coverage.sh";
          };

          # The tree is gofmt-clean.
          gofmt = pkgs.runCommand "nivis-gofmt" { nativeBuildInputs = [ pkgs.go ]; } ''
            cd ${./.}
            export HOME=$TMPDIR
            bad="$(gofmt -l cmd internal tests)"
            if [ -n "$bad" ]; then
              echo "gofmt: these files are not formatted:" >&2
              echo "$bad" >&2
              exit 1
            fi
            touch $out
          '';
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
        # A hermetic tour of the daily-driver features (variables, datasource,
        # round trip, outputs) against the fakes — see docs/TUTORIAL-FEATURES.md.
        # The config is the features-0.4 tutorial starter (also scaffolded by
        # `nivistutor`); kept as `nivis.tutorial` so the milestone-notes golden
        # gate and the docs include keep referring to one place.
        tutorial = import ./nix/example/tutorial-features-0.4/config.nix { inherit nivis; };
        # Remote state on real S3 (M2/B1+B2): fake resources, but state stored in
        # an S3 object with locking. Set the bucket/region in the file first; drive
        # with `AWS_PROFILE=… nivis … --attr nivis.remoteState`. See
        # docs/TUTORIAL-REMOTE-STATE.md.
        remoteState = import ./nix/example/remote-state.nix { inherit nivis; };
        # The self-managed-state-bucket bootstrap: an s3 backend whose location
        # comes from variables, for the `apply --backend=local` -> `state migrate
        # --to-remote` -> `apply` sequence. Fake resources; used by the bootstrap
        # e2e and by docs/REMOTE-STATE.md.
        bootstrapState = import ./nix/example/bootstrap-state.nix { inherit nivis; };
        # A real-provider example (AWS S3 bucket) — drive with `nivis ... --attr
        # nivis.aws`; creates a real resource (see nix/example/aws.nix).
        aws = import ./nix/example/aws.nix { inherit nivis; };
        # EC2 + NixOS: BUILD a NixOS AMI in Nix and launch it (full AWS pipeline
        # as Nivis resources) — the OS image and the infra in one expression. The
        # image's .vhd path flows into aws_s3_object.source. Evaluating this forces
        # nixpkgs (it builds the image). See docs/TUTORIAL-EC2-NIXOS.md.
        ec2 = import ./nix/example/ec2.nix {
          inherit nivis;
          nixosImage = ec2NixosImage;
        };
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
          # The in-repo fake providers, for the offline tutorials (no cloud):
          # `nix build .#fake-providers` -> ./result/bin/provider-{alpha,beta}.
          fake-providers = mkFakeProviders system;
          # The tutorial scaffolder + the fake providers in one /bin, so
          # `nix shell .#nivis .#tutor` carries nivis + nivistutor + the
          # providers a scaffolded tutorial needs. `nix run .#tutor` scaffolds.
          tutor = mkTutor system;
          # The NixOS amazon image for the EC2 tutorial. `nivis` evaluates the
          # config but does not build derivations, so the image must be realised
          # before `nivis apply` uploads it: `nix build .#ec2-image`. Its store
          # path is exactly the one nivis.ec2 feeds to aws_s3_object.source.
          ec2-image = ec2NixosImage;
        }
      );

      # `nix flake check` runs these: build, tests, coverage floors, gofmt.
      checks = forAllSystems mkChecks;

      # CLI app: `nix run .#nivis -- …` (codegen: `nix run .#nivis -- gen …`).
      apps = forAllSystems (
        system:
        let
          cli = mkCli system;
          tutor = mkTutor system;
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
          # `nix run .#tutor` scaffolds a tutorial into your directory.
          tutor = {
            type = "app";
            program = "${tutor}/bin/nivistutor";
          };
        }
      );
    };
}
