# The real-provider tier: nivis applies against an unmodified third-party
# provider (cyrilgdn/postgresql) and a real postgresql, inside a VM with no
# network and no credentials.
#
# It exists because the in-repo fakes encode this project's own reading of the
# provider protocol, so a misreading yields a fake that agrees with it and a
# green suite over a broken path. Four defects found in October 2026
# (nixform2-1mk0, aur6, msl4, cjxz) were all of that shape, and all were found
# by hand against production AWS. A third-party provider decides plan and apply
# semantics from its own schema and cannot share our misreadings.
#
# This is NOT part of `nix flake check`. `scripts/ship-change.sh` runs that on
# every change shipped, and booting a VM there would be paid per ship rather
# than per push. It is built as its own CI step instead. Note this is a
# DIFFERENT reason from the other separate steps (tests/run-nix-tests.sh,
# tests/check-docs-ssot.sh), which are separate because they cannot run in the
# flake sandbox at all. This one could, and should not.
#
# The provider reaches the VM as a store path and the fixture names it as a
# filesystem `source`, so the registry is deliberately not exercised here.
{
  pkgs,
  self,
  nivisCli,
}:
let
  providerPkg = pkgs.terraform-providers.postgresql;
  providerBin = "${providerPkg}/libexec/terraform-providers/registry.terraform.io/cyrilgdn/postgresql/${providerPkg.version}/linux_amd64/terraform-provider-postgresql_${providerPkg.version}";
in
pkgs.testers.runNixOSTest {
  name = "nivis-real-postgres-provider";

  nodes.machine =
    { ... }:
    {
      services.postgresql = {
        enable = true;
        # The provider connects over TCP to localhost. Trust auth, because this
        # test is about the provider protocol and not about authentication: a
        # password here would only add a way for it to fail for an unrelated
        # reason. Nothing outside the VM can reach it.
        enableTCPIP = true;
        authentication = ''
          local all all              trust
          host  all all 127.0.0.1/32 trust
          host  all all ::1/128      trust
        '';
      };

      # Everything the run needs, in the VM's store: the CLI, the provider, the
      # flake source nivis evaluates, and nix itself to evaluate it.
      environment.systemPackages = [
        nivisCli
        pkgs.nix
        pkgs.postgresql
      ];

      # `nix eval <flake>#nivis.postgres` needs the flake source AND its locked
      # inputs present, since the VM cannot fetch them.
      virtualisation.additionalPaths = [
        self
        self.inputs.nixpkgs
        providerPkg
      ];

      virtualisation.memorySize = 2048;
      virtualisation.diskSize = 4096;

      nix.settings.experimental-features = [
        "nix-command"
        "flakes"
      ];
    };

  testScript = ''
    machine.start()
    machine.wait_for_unit("postgresql.target")

    # A role the fixture declares must NOT exist before nivis runs, or the
    # clean-plan assertion later could pass for the wrong reason.
    machine.fail("psql -U postgres -tAc \"SELECT 1 FROM pg_roles WHERE rolname='appuser'\" | grep -q 1")

    nivis = "${nivisCli}/bin/nivis"
    flake = "${self}"
    provider = "${providerBin}"
    common = f"--flake {flake} --attr nivis.postgres --var provider_path={provider} --state /tmp/state.json"

    with subtest("nivis applies against the real provider"):
        machine.succeed(f"cd /tmp && {nivis} apply {common} 2>&1 | tee /tmp/apply.log")

    with subtest("the role exists in postgresql, asked of postgres directly"):
        # Queried through psql, not through nivis, so the assertion does not
        # depend on the thing under test.
        machine.succeed("psql -U postgres -tAc \"SELECT 1 FROM pg_roles WHERE rolname='appuser'\" | grep -q 1")

    with subtest("the plan straight after applying reports no changes"):
        # The acceptance criterion. It is the strongest cheap statement that
        # nivis and a real provider agree about what was applied, and it is what
        # the import work (nixform2-3utj) will reuse.
        out = machine.succeed(f"cd /tmp && {nivis} plan {common} 2>&1 | tee /tmp/plan.log")
        if "No changes" not in out:
            raise Exception("plan after apply was not clean:\n" + out)

    with subtest("import adopts a role created outside nivis"):
        # The acceptance criterion for nixform2-3utj, against a REAL provider:
        # postgresql_role uses ImportStatePassthroughContext, so this exercises
        # the stub-then-read flow against something that does not share our
        # assumptions.
        #
        # The role already exists in postgres and is absent from the FRESH state
        # file used below, which is the migration scenario exactly: existing
        # infrastructure that nivis does not yet track. The imported object must
        # match what the configuration declares, or the plan afterwards reports a
        # real difference and the test would be asserting the wrong thing.
        machine.succeed(
            "cd /tmp && cat > /tmp/import.json <<'EOF'\n"
            '{"imports":{"postgresql.postgresql_role.appuser":{"id":"appuser"}}}\n'
            "EOF"
        )
        # A fresh state file, so the import is not refused as already-in-state.
        imp = f"--flake {flake} --attr nivis.postgres --var provider_path={provider} --state /tmp/import-state.json"
        machine.succeed(f"cd /tmp && {nivis} import {imp} --manifest /tmp/import.json 2>&1 | tee /tmp/import.log")

        out = machine.succeed(f"cd /tmp && {nivis} plan {imp} 2>&1 | tee /tmp/import-plan.log")
        if "No changes" not in out:
            raise Exception("plan after importing was not clean:\n" + out)

    with subtest("import refuses an id that matches nothing"):
        machine.succeed(
            "cd /tmp && cat > /tmp/bad.json <<'EOF'\n"
            '{"imports":{"postgresql.postgresql_role.appuser":{"id":"no-such-role"}}}\n'
            "EOF"
        )
        bad = f"--flake {flake} --attr nivis.postgres --var provider_path={provider} --state /tmp/bad-state.json"
        out = machine.fail(f"cd /tmp && {nivis} import {bad} --manifest /tmp/bad.json 2>&1")
        if "REFUSED" not in out:
            raise Exception("importing a non-existent object was not refused:\n" + out)

    with subtest("the clean plan is a real signal, not a constant"):
        # Without this, the assertion above could pass for a nivis that reports
        # "No changes" unconditionally. Change the role behind nivis's back and
        # the next plan must see it, which is the only thing that makes the
        # clean plan meaningful.
        machine.succeed("psql -U postgres -c 'ALTER ROLE appuser NOLOGIN'")
        out = machine.succeed(f"cd /tmp && {nivis} plan {common} 2>&1 | tee /tmp/drift.log")
        if "No changes" in out:
            raise Exception(
                "plan reported no changes after the role was altered out of band, "
                "so the clean-plan assertion proves nothing:\n" + out
            )
  '';
}
