# Tutorial: a NixOS machine on EC2

This goes further than the [S3 tutorial](TUTORIAL-AWS-S3.md): you **build a NixOS
image in Nix**, register it as an AMI, and launch it as an EC2 instance — and the
*entire* AWS pipeline (upload, import, register, launch) is driven by Nivis. The
machine runs a tiny web server, and you verify the running instance answers
**HTTP 200 on port 80**, then tear it all down.

> ⚠️ **This creates real, billable AWS resources** — an EBS snapshot, an AMI, and
> a `t3.micro` instance — and uploads a ~2 GB image to S3. The walkthrough
> destroys everything at the end. Credentials come from the environment
> (`AWS_PROFILE`); the region is in the Nix config.

The shape:

```
nix build  ──►  NixOS amazon image (a .vhd, nginx baked in)
                      │
   Nivis ── aws_iam_role + policy (vmimport)      the VM-import service role
         ── aws_s3_bucket + aws_s3_object         the .vhd uploaded to S3
         ── aws_ebs_snapshot_import               S3 .vhd → EBS snapshot
         ── aws_ami                               register the snapshot as an AMI
         ── aws_security_group                    ingress :80
         ── aws_instance                          launch it  (public_ip → Nix)
                      │
   curl http://<public_ip>/  ──►  200
```

This mirrors [elastinix](https://github.com/wearetechnative/elastinix) (the
wearetechnative NixOS-on-AWS flake) and its
[Terraform module](https://github.com/wearetechnative/terraform-aws-module-elastinix) —
but driven by Nivis instead of a Terraform module.

## Part 1 — The OS and the infra in one file

The key idea: **the image and the infrastructure live in the same Nix file.** A
NixOS "amazon image" is itself a Nix derivation — `config.system.build.images.amazon`,
a `.vhd` disk image of a machine configuration — so you reference its *build
output* directly as `aws_s3_object.source`. When `nivis apply` evaluates the
flake, Nix realises the image as part of evaluation and its store path flows
straight into the upload. One expression defines the OS **and** the cloud
resources that ship it — that two-domain mix is the whole point of this tutorial.

```nix
{
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.05";
  inputs.nivis.url = "github:wearetechnative/nivis";

  outputs = { self, nixpkgs, nivis }:
    let
      # --- domain 1: the OS, built in Nix (nginx, returns 200 on :80) -----
      image = (nixpkgs.lib.nixosSystem {
        system = "x86_64-linux";
        modules = [ ({ modulesPath, ... }: {
          imports = [ (modulesPath + "/virtualisation/amazon-image.nix") ];
          services.nginx.enable = true;
          services.nginx.virtualHosts."_".locations."/".return =
            ''200 "hello from NixOS on EC2, built and launched by Nivis\n"'';
          networking.firewall.allowedTCPPorts = [ 80 ];
          system.stateVersion = "25.05";
        }) ];
      }).config.system.build.images.amazon;

      # --- domain 2: the infra, as Nivis resources, fed by that image -----
      pipeline = import (nivis + "/nix/example/ec2.nix") {
        nivis = nivis.lib;
        nixosImage = image;   # its .vhd path becomes aws_s3_object.source
      };
    in {
      nivis.plan = ledger: pipeline (ledger // { vars.suffix = "demo"; });
    };
}
```

`nivis apply` builds `image` first (the heavy step — the one that uses the Nix
binary cache, ≈2 GB), then drives the AWS pipeline; everything after the build is
pure AWS. (This repo's `nix/example/ec2.nix` + the `nivis.ec2` flake attr are
exactly this, ready to run.)

## Part 2 — The Nivis pipeline

The example `nix/example/ec2.nix` (flake attr `nivis.ec2`) expresses the whole
AWS side. Its resources, and how they wire together:

- **`aws_iam_role` + `aws_iam_policy` + attachment** — the `vmimport` role AWS
  requires to import a disk image (trust `vmie.amazonaws.com`; S3-read + EBS
  import permissions).
- **`aws_s3_bucket` + `aws_s3_object`** — upload the `.vhd`.
- **`aws_ebs_snapshot_import`** — turn the S3 `.vhd` into an EBS snapshot
  (`role_name` = the vmimport role; `disk_container.user_bucket` = the upload).
- **`aws_ami`** — register the snapshot as a bootable AMI (`ebs_block_device`
  with the snapshot id).
- **`aws_security_group`** — ingress on port 80.
- **`aws_instance`** — launch the AMI (`t3.micro`, the SG attached).

The `aws_s3_object`'s `source` is the built image's `.vhd` path (Part 1) — the OS
crossing into the infra. Each later step references the previous one's output (a
`__ref`), so Nivis resolves the chain across phases: the snapshot import waits on
the upload, the AMI on the snapshot, the instance on the AMI. The only knob your
flake sets is a unique `suffix` (for resource names).

## Part 3 — Build the image, then apply

One wrinkle: `nivis` *evaluates* your flake (it runs `nix eval`) — it does **not
build** derivations. So the image's `.vhd` must be **realised before apply**, or
the S3 upload fails with `no such file or directory`. Build it first (this repo
exposes it as `ec2-image`; in your own flake, expose the `image` derivation as a
package the same way):

```sh
nix build .#ec2-image        # realises the ~2 GB .vhd (the heavy step)
```

Then apply:

```sh
export AWS_PROFILE=your-profile
nivis plan      # 9 resources to create across phases
nivis apply     # upload (~2 GB), import the snapshot (minutes), register, launch
```

> A single `nivis apply` that realises store paths itself is planned
> (beans-qcwb); until then, `nix build` the image first.

A real run of this pipeline (verified against AWS) resolves across four phases —
the AWS chain can't all happen at once:

```
Applied 9 resource(s) across 4 phase(s):
  ✓ aws.aws_iam_role.vmimport
  ✓ aws.aws_iam_policy.vmimport
  ✓ aws.aws_s3_bucket.image
  ✓ aws.aws_security_group.web
  ✓ aws.aws_iam_role_policy_attachment.vmimport
  ✓ aws.aws_s3_object.image          # the ~2 GB NixOS .vhd
  ✓ aws.aws_ebs_snapshot_import.nixos
  ✓ aws.aws_ami.nixos
  ✓ aws.aws_instance.web
```

Read the instance's public address back out of state and check it serves:

```sh
nivis state show aws.aws_instance.web    # public_ip / public_dns / instance_state
curl -sS -o /dev/null -w '%{http_code}\n' "http://<public_ip>/"
# 200
```

The instance boots, `nginx` comes up on port 80, and returns **200** — a machine
whose OS you built in Nix, registered as an AMI through Nivis, and launched, all
from one flake. (Give it a minute after `apply`: the instance has to boot before
nginx answers.)

That `public_ip` did not exist until AWS launched the instance — it was read back
into state (and is available to Nix for dependent config). The instance is
running an OS **you built in Nix**, from an image **you registered through Nivis**.

Tear it all down (reverse dependency order — instance, AMI, snapshot, bucket,
role):

```sh
nivis destroy
```

## Notes

- **Cost & safety:** a `t3.micro` is cheap, but don't leave it running; `nivis
  destroy` removes everything this created. The EBS snapshot import takes a few
  minutes — that's AWS, not Nivis.
- **The `vmimport` role:** AWS requires this specific service role for disk-image
  import; the example creates it (and a least-privilege policy) so the pipeline is
  self-contained. If your account already has a `vmimport` role, point
  `aws_ebs_snapshot_import.role_name` at it instead.
- **Production:** for a real fleet, use elastinix — it owns the image-build +
  upload pipeline and a maintained module. This tutorial shows the mechanism, end
  to end, driven entirely by Nivis.
