# A REAL-provider example that mixes the two domains in ONE Nix file: it BUILDS a
# NixOS amazon image (an OS, in Nix) AND declares the whole AWS pipeline that
# uploads, registers, and launches it (infra, as Nivis resources). The built
# image's store path flows directly into aws_s3_object.source — NixOS-the-OS feeds
# Terraform-the-infra, in a single expression.
#
#   config.system.build.images.amazon   -> a NixOS .vhd (nginx, returns 200)
#   aws_iam_role + policy (vmimport)     -> the VM-import service role
#   aws_s3_bucket + aws_s3_object        -> the VHD (the Nix build output) -> S3
#   aws_ebs_snapshot_import              -> S3 VHD -> EBS snapshot
#   aws_ami                              -> register the snapshot as an AMI
#   aws_security_group                   -> ingress :80
#   aws_instance                         -> launch the AMI (round trip: public_ip)
#
# Building the image needs nixpkgs (unlike the pure Nivis library); that is
# inherent to "NixOS as image". `pkgs` is taken as an arg (the flake passes
# nixpkgs); only `suffix` (a unique name) comes from the ledger's vars.
{
  nivis,
  # The NixOS amazon image derivation (the flake supplies it). When absent (a
  # pure IR eval in tests) source falls back to a placeholder path — only the AWS
  # resource shapes are exercised.
  nixosImage ? null,
}:
ledger:
let
  inherit (nivis) mkResource mkProvider toIR drv;

  vars = ledger.vars or { };
  suffix = vars.suffix or "demo";
  # The image is a Nix BUILD OUTPUT, marked with `drv`: a __build leaf the
  # executor realises (builds) before uploading — the OS crossing into the infra,
  # no manual store-path interpolation. `drv` uses the image's passthru.filePath
  # (the .vhd) automatically. A pure IR eval (no nixpkgs/image) uses a placeholder.
  imgSource = if nixosImage != null then drv nixosImage else (vars.imagePath or "/path/to/nixos.vhd");
  bucketName = "nivis-ec2nix-${suffix}";
  amiName = "nivis-ec2nix-${suffix}";

  vmimportPolicy = builtins.toJSON {
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

  vmimportTrust = builtins.toJSON {
    Version = "2012-10-17";
    Statement = [
      {
        Effect = "Allow";
        Principal.Service = "vmie.amazonaws.com";
        Action = "sts:AssumeRole";
        Condition.StringEquals."sts:Externalid" = "vmimport";
      }
    ];
  };

  role = mkResource {
    provider = "aws"; type = "aws_iam_role"; name = "vmimport";
    config = { name = "nivis-vmimport-${suffix}"; assume_role_policy = vmimportTrust; };
  };

  policy = mkResource {
    provider = "aws"; type = "aws_iam_policy"; name = "vmimport";
    config = { name = "nivis-vmimport-${suffix}"; policy = vmimportPolicy; };
  };

  attach = mkResource {
    provider = "aws"; type = "aws_iam_role_policy_attachment"; name = "vmimport";
    config = { role = role.refAttr "name"; policy_arn = policy.refAttr "arn"; };
  };

  bucket = mkResource {
    provider = "aws"; type = "aws_s3_bucket"; name = "image";
    config = { bucket = bucketName; force_destroy = true; };
  };

  image = mkResource {
    provider = "aws"; type = "aws_s3_object"; name = "image";
    config = { bucket = bucket.refAttr "id"; key = "nixos.vhd"; source = imgSource; };
  };

  snapshot = mkResource {
    provider = "aws"; type = "aws_ebs_snapshot_import"; name = "nixos";
    config = {
      role_name = role.refAttr "name";
      # disk_container and user_bucket are LIST-nested blocks in the AWS provider,
      # so each takes a one-element list (a bare attrset is rejected at apply).
      disk_container = [
        {
          format = "VHD";
          user_bucket = [ { s3_bucket = bucket.refAttr "id"; s3_key = "nixos.vhd"; } ];
        }
      ];
    };
  };

  ami = mkResource {
    provider = "aws"; type = "aws_ami"; name = "nixos";
    config = {
      name = amiName;
      virtualization_type = "hvm";
      root_device_name = "/dev/xvda";
      ena_support = true;
      ebs_block_device = [
        { device_name = "/dev/xvda"; snapshot_id = snapshot.refAttr "id"; }
      ];
    };
  };

  sg = mkResource {
    provider = "aws"; type = "aws_security_group"; name = "web";
    config = {
      name = "nivis-ec2nix-web-${suffix}";
      description = "Nivis EC2+NixOS demo: allow HTTP";
      ingress = [
        { from_port = 80; to_port = 80; protocol = "tcp"; cidr_blocks = [ "0.0.0.0/0" ]; description = "http"; }
      ];
      egress = [
        { from_port = 0; to_port = 0; protocol = "-1"; cidr_blocks = [ "0.0.0.0/0" ]; description = "all"; }
      ];
    };
  };

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
