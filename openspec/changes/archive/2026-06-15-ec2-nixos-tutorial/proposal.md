# Proposal: ec2-nixos-tutorial

## Why
The AWS tutorial covers S3 (storage). The next step infra users want is a
**running machine** — and the Nix-shaped version of that is **NixOS on EC2**:
build the machine *and its image* in Nix, then launch it. beans-rx5h asks for an
EC2-with-NixOS tutorial, and explicitly that **its outcome be tested**. This adds
that, end to end, with a concrete success check: the instance serves **HTTP 200
on port 80**.

`aws_instance` already introspects cleanly through Nivis's codec (verified via
`nivis gen`), so the executor side is feasible; the tutorial wires it together.

## What changes
- **`docs/TUTORIAL-EC2-NIXOS.md`** — a guided walkthrough:
  1. **Build & upload a NixOS AMI with [elastinix](https://github.com/wearetechnative/elastinix)**
     (the wearetechnative flake that builds NixOS images via the nixpkgs image
     system and registers them as AMIs). The machine config enables a **minimal
     HTTP server** (nginx or a tiny static server) on port 80. Link elastinix for
     the build/upload specifics; the tutorial shows the config knob that matters
     (the HTTP service) and the resulting AMI id.
  2. **Launch it with Nivis** — a flake exposing `nivis.plan` with an
     `aws_security_group` (ingress :80) and an `aws_instance` (`ami` = the
     elastinix AMI, `instance_type = t3.micro`, the SG attached), driven by
     `nivis apply`. Read `public_ip`/`public_dns` back into Nix (the round trip).
  3. **Verify** — `curl http://<public_ip>/` returns **200**; then
     `nivis destroy`.
- **A gated Go e2e** (`internal/plugin/ec2_nixos_aws_test.go`, behind
  `TERRAE_NIVIS_NET_TESTS=1` + an AMI id env): launch the instance via the
  provider client, poll port 80 until **HTTP 200** (with a timeout), then destroy
  and confirm no orphan. This is the bean's "tested outcome."
- Wire the tutorial into the docs site (a `docs-site/src` page + nav) and the
  docs-SSOT convention.

## Non-goals
- Re-implementing elastinix's image build/upload inside this repo — the AMI build
  is **delegated to elastinix** (it owns the nixpkgs-image → AMI pipeline, which
  is cache-heavy); the tutorial references it. Nivis's part is the launch + check.
- A NixOS module library in Nivis — the HTTP server is a one-line elastinix/NixOS
  config, not a Nivis feature.
- Networking beyond a default-VPC public-subnet instance + an SG — no custom VPC.

## Impact
- New: `docs/TUTORIAL-EC2-NIXOS.md`, the site page + nav, a gated EC2 e2e.
  Changed: docs-SSOT table/check; possibly `nix/example` gains an EC2 example
  attr if useful for the e2e.
- Verification: `aws_instance` + `aws_security_group` plan/apply through Nivis;
  the gated e2e launches a real t3.micro, asserts `:80 → 200`, destroys it (no
  orphan). Cost is minutes of t3.micro time. **Note:** the AMI build (elastinix)
  needs the Nix binary cache; if unreachable in a given environment, the tutorial
  documents the AMI as a prerequisite and the e2e takes the AMI id as input.
- Closes beans-rx5h.
