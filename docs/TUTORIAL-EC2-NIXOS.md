# Tutorial: a NixOS machine on EC2

This goes further than the [S3 tutorial](TUTORIAL-AWS-S3.md): you **build a NixOS
image in Nix**, register it as an AMI, and launch it as an EC2 instance, and the
*entire* AWS pipeline (upload, import, register, launch) is driven by Nivis. The
machine runs a tiny web server, and you verify the running instance answers
**HTTP 200 on port 80**, then tear it all down.

> ⚠️ **This creates real, billable AWS resources** (an EBS snapshot, an AMI, and
> a `t3.micro` instance) and uploads a ~2 GB image to S3. The walkthrough
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
[Terraform module](https://github.com/wearetechnative/terraform-aws-module-elastinix),
but driven by Nivis instead of a Terraform module.

## Part 1: The OS and the infra in one file

The key idea: **the image and the infrastructure live in the same Nix file.** A
NixOS "amazon image" is itself a Nix derivation, `config.system.build.images.amazon`,
a `.vhd` disk image of a machine configuration, so you reference its *build
output* directly as `aws_s3_object.source`. When `nivis apply` evaluates the
flake, Nix realises the image as part of evaluation and its store path flows
straight into the upload. One expression defines the OS **and** the cloud
resources that ship it: that two-domain mix is the whole point of this tutorial.

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
        nixosImage = image;   # `drv image` -> aws_s3_object.source (realised at apply)
      };
    in {
      nivis.plan = ledger: pipeline (ledger // { vars.suffix = "demo"; });
    };
}
```

`nivis apply` builds `image` first (the heavy step, the one that uses the Nix
binary cache, ≈2 GB), then drives the AWS pipeline; everything after the build is
pure AWS. (This repo's `nix/example/ec2.nix` + the `nivis.ec2` flake attr are
exactly this, ready to run.)

## Part 2: The Nivis pipeline

Here is the whole AWS side: a function of the built image and the outputs
ledger, returning the IR. It's the contents of this repo's `nix/example/ec2.nix`
(exposed as the flake attr `nivis.ec2`); drop it in your own repo and import it as
shown in Part 1, or inline it.

```nix
{
  nivis,        # the Nivis library (mkResource / mkProvider / toIR / drv)
  nixosImage,   # the NixOS amazon image derivation from Part 1
}:
ledger:
let
  inherit (nivis) mkResource mkProvider toIR drv;

  suffix = (ledger.vars or { }).suffix or "demo";
  bucketName = "nivis-ec2nix-${suffix}";

  # The OS crossing into the infra: `drv` marks the image as a build output,
  # a __build leaf that `nivis apply` realises (builds) before uploading, then
  # substitutes the concrete .vhd path. No manual store-path interpolation; no
  # separate `nix build`. (drv uses the image's passthru.filePath, the .vhd.)
  imgSource = drv nixosImage;

  # --- the vmimport service role AWS requires to import a disk image ---------
  role = mkResource {
    provider = "aws"; type = "aws_iam_role"; name = "vmimport";
    config = {
      name = "nivis-vmimport-${suffix}";
      assume_role_policy = builtins.toJSON {
        Version = "2012-10-17";
        Statement = [{
          Effect = "Allow";
          Principal.Service = "vmie.amazonaws.com";
          Action = "sts:AssumeRole";
          Condition.StringEquals."sts:Externalid" = "vmimport";
        }];
      };
    };
  };

  policy = mkResource {
    provider = "aws"; type = "aws_iam_policy"; name = "vmimport";
    config = {
      name = "nivis-vmimport-${suffix}";
      policy = builtins.toJSON {
        Version = "2012-10-17";
        Statement = [
          {
            Effect = "Allow";
            Action = [ "s3:GetBucketLocation" "s3:GetObject" "s3:ListBucket" "s3:PutObject" "s3:GetBucketAcl" ];
            Resource = [ "arn:aws:s3:::${bucketName}" "arn:aws:s3:::${bucketName}/*" ];
          }
          {
            Effect = "Allow";
            Action = [ "ec2:ModifySnapshotAttribute" "ec2:CopySnapshot" "ec2:RegisterImage" "ec2:Describe*" ];
            Resource = "*";
          }
        ];
      };
    };
  };

  attach = mkResource {
    provider = "aws"; type = "aws_iam_role_policy_attachment"; name = "vmimport";
    config = { role = role.refAttr "name"; policy_arn = policy.refAttr "arn"; };
  };

  # --- upload the built .vhd to S3 ------------------------------------------
  bucket = mkResource {
    provider = "aws"; type = "aws_s3_bucket"; name = "image";
    config = { bucket = bucketName; force_destroy = true; };
  };

  image = mkResource {
    provider = "aws"; type = "aws_s3_object"; name = "image";
    config = { bucket = bucket.refAttr "id"; key = "nixos.vhd"; source = imgSource; };
  };

  # --- S3 .vhd -> EBS snapshot ----------------------------------------------
  # disk_container and user_bucket are LIST-nested blocks, so each is a
  # one-element list (a bare attrset is rejected at apply).
  snapshot = mkResource {
    provider = "aws"; type = "aws_ebs_snapshot_import"; name = "nixos";
    config = {
      role_name = role.refAttr "name";
      disk_container = [{
        format = "VHD";
        user_bucket = [{ s3_bucket = bucket.refAttr "id"; s3_key = "nixos.vhd"; }];
      }];
    };
  };

  # --- register the snapshot as a bootable AMI ------------------------------
  ami = mkResource {
    provider = "aws"; type = "aws_ami"; name = "nixos";
    config = {
      name = "nivis-ec2nix-${suffix}";
      virtualization_type = "hvm";
      root_device_name = "/dev/xvda";
      ena_support = true;
      ebs_block_device = [{ device_name = "/dev/xvda"; snapshot_id = snapshot.refAttr "id"; }];
    };
  };

  # --- a security group opening port 80 -------------------------------------
  sg = mkResource {
    provider = "aws"; type = "aws_security_group"; name = "web";
    config = {
      name = "nivis-ec2nix-web-${suffix}";
      description = "Nivis EC2+NixOS demo: allow HTTP";
      ingress = [{ from_port = 80; to_port = 80; protocol = "tcp"; cidr_blocks = [ "0.0.0.0/0" ]; description = "http"; }];
      egress  = [{ from_port = 0;  to_port = 0;  protocol = "-1";  cidr_blocks = [ "0.0.0.0/0" ]; description = "all";  }];
    };
  };

  # --- launch the AMI -------------------------------------------------------
  instance = mkResource {
    provider = "aws"; type = "aws_instance"; name = "web";
    config = {
      ami = ami.refAttr "id";
      instance_type = "t3.micro";
      vpc_security_group_ids = [ (sg.refAttr "id") ];
      tags = { Name = "nivis-ec2nix-${suffix}"; managed-by = "nivis"; };
    };
  };
in
toIR {
  providers.aws = mkProvider {
    source = "registry.opentofu.org/hashicorp/aws";
    config = { region = "eu-central-1"; };
  };
  resources = [ role policy attach bucket image snapshot ami sg instance ];
  inherit ledger;
}
```

Reading the chain: `image`'s `source` is the built `.vhd` path (Part 1): the OS
crossing into the infra. Every later resource references the previous one's output
with `refAttr` (a `__ref`), so Nivis resolves the chain across phases: the
snapshot import waits on the upload, the AMI on the snapshot, the instance on the
AMI. The IAM role + policy create the `vmimport` service role AWS requires for
disk-image import. The only per-deployment knob is `suffix` (unique resource
names).

## Part 3: Apply

```sh
export AWS_PROFILE=your-profile
nivis plan      # 9 resources to create across phases
nivis apply     # build the image, upload (~2 GB), import, register, launch
```

Because `source` is a `drv` (`__build`) leaf, **`nivis apply` builds the image
itself** before uploading it: no separate `nix build` step. The image build is
the heavy part (it uses the Nix binary cache, ≈2 GB); everything after is pure
AWS. (Pass `--build=false` to skip realising if you've pre-built.)

A real run of this pipeline (verified against AWS) resolves across four phases:
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

The instance boots, `nginx` comes up on port 80, and returns **200**: a machine
whose OS you built in Nix, registered as an AMI through Nivis, and launched, all
from one flake. (Give it a minute after `apply`: the instance has to boot before
nginx answers.)

That `public_ip` did not exist until AWS launched the instance; it was read back
into state (and is available to Nix for dependent config). The instance is
running an OS **you built in Nix**, from an image **you registered through Nivis**.

Tear it all down (reverse dependency order: instance, AMI, snapshot, bucket,
role):

```sh
nivis destroy
```

## Notes

- **Cost & safety:** a `t3.micro` is cheap, but don't leave it running; `nivis
  destroy` removes everything this created. The EBS snapshot import takes a few
  minutes; that's AWS, not Nivis.
- **The `vmimport` role:** AWS requires this specific service role for disk-image
  import; the example creates it (and a least-privilege policy) so the pipeline is
  self-contained. If your account already has a `vmimport` role, point
  `aws_ebs_snapshot_import.role_name` at it instead.
- **Production:** for a real fleet, use elastinix: it owns the image-build +
  upload pipeline and a maintained module. This tutorial shows the mechanism, end
  to end, driven entirely by Nivis.
