# A minimal REAL-provider example: one AWS S3 bucket via the hashicorp/aws
# provider (resolved + verified + cached from the OpenTofu registry by the
# executor). Drive it with the tn CLI:
#
#   AWS_PROFILE=<profile> AWS_REGION=<region> tn apply   --attr terraeNivis.aws
#   tn state show aws.aws_s3_bucket.demo
#   AWS_PROFILE=<profile> AWS_REGION=<region> tn destroy --attr terraeNivis.aws
#
# This CREATES a real (free-tier) S3 bucket and then destroys it. Credentials and
# region come from the environment (the AWS SDK default chain) — Nix-side provider
# config is a separate item (beans-prj4). `bucket` is omitted so AWS assigns a
# globally-unique name; force_destroy lets `tn destroy` delete it unconditionally.
{ terraeNivis }:
ledger:
let
  inherit (terraeNivis) mkResource toIR;

  bucket = mkResource {
    provider = "aws";
    type = "aws_s3_bucket";
    name = "demo";
    config = {
      force_destroy = true;
      tags = { nixform-test = "terrae-nivis-aws-example"; };
    };
  };
in
toIR {
  providers = {
    aws = {
      source = "registry.opentofu.org/hashicorp/aws";
      config = { }; # region/creds from the environment (AWS SDK default chain)
    };
  };
  resources = [ bucket ];
  inherit ledger;
}
